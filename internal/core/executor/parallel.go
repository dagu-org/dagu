package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/scheduler"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ scheduler.ParallelExecutor = (*parallelExecutor)(nil)
var _ scheduler.NodeStatusDeterminer = (*parallelExecutor)(nil)

type parallelExecutor struct {
	child         *ChildDAGExecutor
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	runParamsList []scheduler.RunParams
	maxConcurrent int

	// Runtime state
	running    map[string]*exec.Cmd       // Maps DAG run ID to running command
	results    map[string]*core.RunStatus // Maps DAG run ID to result
	errors     []error                    // Collects errors from failed executions
	wg         sync.WaitGroup             // Tracks running goroutines
	cancelFunc context.CancelFunc         // For canceling all child executions
}

func newParallelExecutor(
	ctx context.Context, step core.Step,
) (core.Executor, error) {
	// The parallel executor doesn't use the params from the step directly
	// as they are passed through SetParamsList

	if step.ChildDAG == nil {
		return nil, fmt.Errorf("child DAG configuration is missing")
	}

	child, err := NewChildDAGExecutor(ctx, step.ChildDAG.Name)
	if err != nil {
		return nil, err
	}

	dir := core.GetEnv(ctx).WorkingDir
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
		results:       make(map[string]*core.RunStatus),
		errors:        make([]error, 0),
	}, nil
}

func (e *parallelExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "err", err)
		}
	}()

	if len(e.runParamsList) == 0 {
		return fmt.Errorf("no child DAG runs to execute")
	}

	// Create a cancellable context for all child executions
	ctx, e.cancelFunc = context.WithCancel(ctx)
	defer e.cancelFunc()

	// Create a semaphore channel to limit concurrent executions
	semaphore := make(chan struct{}, e.maxConcurrent)

	// Channel to collect errors from goroutines
	errChan := make(chan error, len(e.runParamsList))

	logger.Info(ctx, "Starting parallel execution",
		"total", len(e.runParamsList),
		"maxConcurrent", e.maxConcurrent,
		"dag", e.child.DAG.Name,
	)

	// Launch all child DAG executions
	for _, params := range e.runParamsList {
		e.wg.Add(1)
		go func(runParams scheduler.RunParams) {
			defer e.wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
				defer func() { <-semaphore }()
			case <-ctx.Done():
				errChan <- ctx.Err()
				return
			}

			// Execute child DAG
			if err := e.executeChild(ctx, runParams); err != nil {
				logger.Error(ctx, "Child DAG execution failed",
					"runId", runParams.RunID,
					"err", err,
				)
				errChan <- fmt.Errorf("child DAG %s failed: %w", runParams.RunID, err)
			}
		}(params)
	}

	// Wait for all executions to complete
	e.wg.Wait()
	close(errChan)

	// Collect all errors
	for err := range errChan {
		e.errors = append(e.errors, err)
	}

	// Always output aggregated results, even if some executions failed
	if err := e.outputResults(ctx); err != nil {
		// Log the output error but don't fail the entire execution because of it
		logger.Error(ctx, "Failed to output results", "err", err)
	}

	// Check if any executions failed
	if len(e.errors) > 0 {
		// Check if any error is due to context cancellation
		for _, err := range e.errors {
			if err == context.Canceled {
				return fmt.Errorf("parallel execution cancelled")
			}
		}
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(e.errors), e.errors[0])
	}

	// Check if any child DAGs failed (even if they completed without execution errors)
	e.lock.Lock()
	failedCount := 0
	for _, result := range e.results {
		if !result.Status.IsSuccess() {
			failedCount++
		}
	}
	e.lock.Unlock()

	if failedCount > 0 {
		return fmt.Errorf("parallel execution failed: %d child dag(s) failed", failedCount)
	}

	return nil
}

func (e *parallelExecutor) SetParamsList(paramsList []scheduler.RunParams) {
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
func (e *parallelExecutor) DetermineNodeStatus(_ context.Context) (status.NodeStatus, error) {
	if len(e.results) == 0 {
		return status.NodeError, fmt.Errorf("no results available for node status determination")
	}

	// Check if all child DAGs succeeded or if any had partial success
	// For error cases, we return an error status with error message
	var partialSuccess bool
	for _, result := range e.results {
		if !result.Status.IsSuccess() {
			return status.NodeError, fmt.Errorf("child DAG run %s failed with status: %s", result.DAGRunID, result.Status)
		}
		if result.Status == status.PartialSuccess {
			partialSuccess = true
		}
	}

	if partialSuccess {
		return status.NodePartialSuccess, nil
	}
	return status.NodeSuccess, nil
}

// executeChild executes a single child DAG with the given parameters
func (e *parallelExecutor) executeChild(ctx context.Context, runParams scheduler.RunParams) error {
	// Use the new ExecuteWithResult API
	result, err := e.child.ExecuteWithResult(ctx, runParams, e.workDir)

	// Store the result
	e.lock.Lock()
	if result != nil {
		e.results[runParams.RunID] = result
	}
	e.lock.Unlock()

	return err
}

// outputResults aggregates and outputs all child DAG results
func (e *parallelExecutor) outputResults(_ context.Context) error {
	e.lock.Lock()
	defer e.lock.Unlock()

	// Create aggregated output
	output := struct {
		Summary struct {
			Total     int `json:"total"`
			Succeeded int `json:"succeeded"`
			Failed    int `json:"failed"`
		} `json:"summary"`
		Results []core.RunStatus    `json:"results"`
		Outputs []map[string]string `json:"outputs"`
	}{}

	output.Summary.Total = len(e.runParamsList)
	output.Results = make([]core.RunStatus, 0, len(e.results))
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

	// Cancel the context to stop new executions
	if e.cancelFunc != nil {
		e.cancelFunc()
	}

	// Kill all child processes (both local and distributed)
	if e.child != nil {
		return e.child.Kill(sig)
	}

	return nil
}

func init() {
	core.RegisterExecutor(core.ExecutorTypeParallel, newParallelExecutor, nil)
}
