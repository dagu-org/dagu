package dag

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/executor"
)

var errParallelCancelled = errors.New("parallel execution cancelled")

var _ executor.ParallelExecutor = (*parallelExecutor)(nil)
var _ executor.NodeStatusDeterminer = (*parallelExecutor)(nil)

type parallelExecutor struct {
	child         *executor.SubDAGExecutor
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	runParamsList []executor.RunParams
	maxConcurrent int

	// Runtime state
	running map[string]*exec.Cmd            // Maps DAG run ID to running command
	results map[string]*execution.RunStatus // Maps DAG run ID to result
	errors  []error                         // Collects errors from failed executions

	cancel     chan struct{}
	cancelOnce sync.Once
}

func newParallelExecutor(
	ctx context.Context, step core.Step,
) (executor.Executor, error) {
	// The parallel executor doesn't use the params from the step directly
	// as they are passed through SetParamsList

	if step.SubDAG == nil {
		return nil, fmt.Errorf("sub DAG configuration is missing")
	}

	child, err := executor.NewSubDAGExecutor(ctx, step.SubDAG.Name)
	if err != nil {
		return nil, err
	}

	dir := execution.GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	maxConcurrent := core.DefaultMaxConcurrent
	if step.Parallel != nil && step.Parallel.MaxConcurrent > 0 {
		maxConcurrent = step.Parallel.MaxConcurrent
	}

	return &parallelExecutor{
		child:         child,
		workDir:       dir,
		maxConcurrent: maxConcurrent,
		running:       make(map[string]*exec.Cmd),
		results:       make(map[string]*execution.RunStatus),
		errors:        make([]error, 0),
		cancel:        make(chan struct{}),
	}, nil
}

func (e *parallelExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup sub DAG executor", "err", err)
		}
	}()

	if len(e.runParamsList) == 0 {
		return fmt.Errorf("no sub DAG runs to execute")
	}

	// Create a semaphore channel to limit concurrent executions
	semaphore := make(chan struct{}, e.maxConcurrent)

	// Channel to collect errors from goroutines
	errChan := make(chan error, len(e.runParamsList))

	logger.Info(ctx, "Starting parallel execution",
		"total", len(e.runParamsList),
		"maxConcurrent", e.maxConcurrent,
		"dag", e.child.DAG.Name,
	)

	// Launch all sub DAG executions
	var wg sync.WaitGroup
	for _, params := range e.runParamsList {
		wg.Add(1)
		go func(runParams executor.RunParams) {
			defer wg.Done()

			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-e.cancel:
				errChan <- errParallelCancelled
				return
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			// Execute sub DAG
			if err := e.executeChild(ctx, runParams); err != nil {
				logger.Error(ctx, "Sub DAG execution failed", "runId", runParams.RunID, "params", runParams, "err", err)
				errChan <- fmt.Errorf("sub DAG %s failed: %w", runParams.RunID, err)
			}
		}(params)
	}

	// Wait for all executions to complete
	wg.Wait()
	close(errChan)

	// Collect all errors
	for err := range errChan {
		e.errors = append(e.errors, err)
	}

	// Always output aggregated results, even if some executions failed
	if err := e.outputResults(); err != nil {
		// Log the output error but don't fail the entire execution because of it
		logger.Error(ctx, "Failed to output results", "err", err)
	}

	// Check if any executions failed
	if len(e.errors) > 0 {
		// Check if any error is due to context cancellation
		for _, err := range e.errors {
			if errors.Is(err, context.Canceled) {
				return fmt.Errorf("parallel execution cancelled")
			}
		}
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(e.errors), e.errors[0])
	}

	// Check if any sub DAGs failed (even if they completed without execution errors)
	e.lock.Lock()
	failedCount := 0
	for _, result := range e.results {
		if !result.Status.IsSuccess() {
			failedCount++
		}
	}
	e.lock.Unlock()

	if failedCount > 0 {
		return fmt.Errorf("parallel execution failed: %d sub dag(s) failed", failedCount)
	}

	return nil
}

func (e *parallelExecutor) SetParamsList(paramsList []executor.RunParams) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.runParamsList = paramsList
}

func (e *parallelExecutor) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *parallelExecutor) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

// DetermineNodeStatus implements NodeStatusDeterminer.
func (e *parallelExecutor) DetermineNodeStatus() (core.NodeStatus, error) {
	if len(e.results) == 0 {
		return core.NodeFailed, fmt.Errorf("no results available for node status determination")
	}

	// Check if all sub DAGs succeeded or if any had partial success
	// For error cases, we return an error status with error message
	var partialSuccess bool
	for _, result := range e.results {
		switch result.Status {
		case core.Succeeded:
			// continue checking other results
		case core.PartiallySucceeded:
			partialSuccess = true
		default:
			return core.NodeFailed, fmt.Errorf("sub DAG run %s is still in progress with status: %s", result.DAGRunID, result.Status)
		}
	}

	// Check count of items equal to count of succeeded items
	if len(e.results) != len(e.runParamsList) {
		partialSuccess = true
	}

	if partialSuccess {
		return core.NodePartiallySucceeded, nil
	}

	return core.NodeSucceeded, nil
}

// executeChild executes a single sub DAG with the given parameters
func (e *parallelExecutor) executeChild(ctx context.Context, runParams executor.RunParams) error {
	result, err := e.child.Execute(ctx, runParams, e.workDir)

	e.lock.Lock()
	if result != nil {
		e.results[runParams.RunID] = result
	}
	e.lock.Unlock()

	return err
}

// outputResults aggregates and outputs all sub DAG results
func (e *parallelExecutor) outputResults() error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Create aggregated output
	output := struct {
		Summary struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
		} `json:"summary"`
		Results []execution.RunStatus `json:"results"`
		Outputs []map[string]string   `json:"outputs"`
	}{}

	output.Summary.Total = len(e.runParamsList)
	output.Results = make([]execution.RunStatus, 0, len(e.results))
	output.Outputs = make([]map[string]string, 0, len(e.results))

	// Collect results in order of runParamsList for consistency
	for _, params := range e.runParamsList {
		if result, ok := e.results[params.RunID]; ok {
			// Create a copy of the result to potentially modify it
			resultCopy := *result

			output.Results = append(output.Results, resultCopy)

			if result.Status.IsSuccess() {
				output.Summary.Succeeded++

				// Add output to the outputs array
				// Only include outputs from successful executions
				if result.Outputs != nil {
					output.Outputs = append(output.Outputs, result.Outputs)
				}
			} else {
				output.Summary.Failed++
			}
		}
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	// Add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	// Write to stdout
	if e.stdout != nil {
		if _, err := e.stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return nil
}

func (e *parallelExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.cancelOnce.Do(func() {
		close(e.cancel)
	})
	if e.child != nil {
		return e.child.Kill(sig)
	}
	return nil
}

func init() {
	executor.RegisterExecutor(core.ExecutorTypeParallel, newParallelExecutor, nil)
}
