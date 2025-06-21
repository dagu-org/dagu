package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ ParallelExecutor = (*parallelExecutor)(nil)

type parallelExecutor struct {
	child         *ChildDAGExecutor
	lock          sync.Mutex
	workDir       string
	stdout        io.Writer
	stderr        io.Writer
	runParamsList []RunParams
	maxConcurrent int

	// Runtime state
	running    map[string]*exec.Cmd    // Maps DAG run ID to command
	results    map[string]*ChildResult // Maps DAG run ID to result
	errors     []error                 // Collects errors from failed executions
	wg         sync.WaitGroup          // Tracks running goroutines
	cancelFunc context.CancelFunc      // For canceling all child executions
}

// ChildResult holds the result of a single child DAG execution
type ChildResult struct {
	RunID    string         `json:"runId"`
	Params   string         `json:"params"`
	Status   string         `json:"status"`
	Output   map[string]any `json:"output,omitempty"`
	Error    string         `json:"error,omitempty"`
	ExitCode int            `json:"exitCode"`
}

func newParallelExecutor(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	// The parallel executor doesn't use the params from the step directly
	// as they are passed through SetParamsList

	if step.ChildDAG == nil {
		return nil, fmt.Errorf("child DAG configuration is missing")
	}

	child, err := NewChildDAGExecutor(ctx, step.ChildDAG.Name)
	if err != nil {
		return nil, err
	}

	dir := GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	maxConcurrent := digraph.DefaultMaxConcurrent
	if step.Parallel != nil && step.Parallel.MaxConcurrent > 0 {
		maxConcurrent = step.Parallel.MaxConcurrent
	}

	return &parallelExecutor{
		child:         child,
		workDir:       dir,
		maxConcurrent: maxConcurrent,
		running:       make(map[string]*exec.Cmd),
		results:       make(map[string]*ChildResult),
		errors:        make([]error, 0),
	}, nil
}

func (e *parallelExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "error", err)
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
		go func(runParams RunParams) {
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
					"error", err,
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
		logger.Error(ctx, "Failed to output results", "error", err)
	}

	// Check if any executions failed
	if len(e.errors) > 0 {
		return fmt.Errorf("parallel execution failed with %d errors: %v", len(e.errors), e.errors[0])
	}

	return nil
}

func (e *parallelExecutor) SetParamsList(paramsList []RunParams) {
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

// executeChild executes a single child DAG with the given parameters
func (e *parallelExecutor) executeChild(ctx context.Context, runParams RunParams) error {
	cmd, err := e.child.BuildCommand(ctx, runParams, e.workDir)
	if err != nil {
		return err
	}

	// Create pipes for stdout/stderr capture
	// We'll collect individual outputs for aggregation
	cmd.Stdout = io.Discard // TODO: Capture individual outputs if needed
	cmd.Stderr = io.Discard // TODO: Capture individual errors if needed

	// Store the command for potential killing
	e.lock.Lock()
	e.running[runParams.RunID] = cmd
	e.lock.Unlock()

	logger.Info(ctx, "Executing child DAG",
		"dagRunId", runParams.RunID,
		"target", e.child.DAG.Name,
		"params", runParams.Params,
	)

	// Start and wait for the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start child dag-run: %w", err)
	}

	err = cmd.Wait()

	// Remove from running map
	e.lock.Lock()
	delete(e.running, runParams.RunID)
	e.lock.Unlock()

	// Get the result regardless of error
	env := GetEnv(ctx)
	result, resultErr := env.DB.GetChildDAGRunStatus(ctx, runParams.RunID, env.RootDAGRun)

	// Store the result
	e.lock.Lock()
	if resultErr == nil && result != nil {
		// Convert digraph.Status outputs to map[string]any
		outputs := make(map[string]any)
		for k, v := range result.Outputs {
			outputs[k] = v
		}

		// Determine status based on execution error
		status := "success"
		if err != nil {
			status = "failed"
		}

		e.results[runParams.RunID] = &ChildResult{
			RunID:    runParams.RunID,
			Params:   runParams.Params,
			Status:   status,
			Output:   outputs,
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	} else {
		// Even if we couldn't get the result, store what we know
		status := "error"
		if err == nil && resultErr != nil {
			// Command succeeded but couldn't get result
			status = "success"
		} else if err != nil {
			status = "failed"
		}

		e.results[runParams.RunID] = &ChildResult{
			RunID:    runParams.RunID,
			Params:   runParams.Params,
			Status:   status,
			Error:    fmt.Sprintf("execution error: %v, result error: %v", err, resultErr),
			ExitCode: cmd.ProcessState.ExitCode(),
		}
	}
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("child dag-run failed: %w", err)
	}

	return nil
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
			Errors    int `json:"errors"`
		} `json:"summary"`
		Results []ChildResult    `json:"results"`
		Outputs []map[string]any `json:"outputs"`
	}{}

	output.Summary.Total = len(e.runParamsList)
	output.Results = make([]ChildResult, 0, len(e.results))
	output.Outputs = make([]map[string]any, 0, len(e.results))

	// Collect results in order of runParamsList for consistency
	for _, params := range e.runParamsList {
		if result, ok := e.results[params.RunID]; ok {
			// Create a copy of the result to potentially modify it
			resultCopy := *result

			// Clear output for failed executions
			if result.Status != "success" {
				resultCopy.Output = nil
			}

			output.Results = append(output.Results, resultCopy)

			// Add output to the outputs array
			// Only include outputs from successful executions
			if result.Status == "success" && result.Output != nil {
				output.Outputs = append(output.Outputs, result.Output)
			}

			switch result.Status {
			case "success":
				output.Summary.Succeeded++
			case "failed", "error":
				output.Summary.Failed++
			}

			if result.Error != "" {
				output.Summary.Errors++
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

	// Kill all running child processes
	var lastErr error
	for _, cmd := range e.running {
		if cmd != nil && cmd.Process != nil {
			if err := syscall.Kill(-cmd.Process.Pid, sig.(syscall.Signal)); err != nil {
				lastErr = err
			}
		}
	}

	return lastErr
}

func init() {
	Register(digraph.ExecutorTypeParallel, newParallelExecutor)
}
