package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/coordinator/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

var _ DAGExecutor = (*dagExecutor)(nil)

type dagExecutor struct {
	child     *ChildDAGExecutor
	lock      sync.Mutex
	workDir   string
	stdout    io.Writer
	stderr    io.Writer
	cmd       *exec.Cmd
	runParams RunParams
	step      digraph.Step
}

// Errors for DAG executor
var (
	ErrWorkingDirNotExist = fmt.Errorf("working directory does not exist")
)

func newDAGExecutor(
	ctx context.Context, step digraph.Step,
) (Executor, error) {
	if step.ChildDAG == nil {
		return nil, fmt.Errorf("child DAG configuration is missing")
	}

	child, err := NewChildDAGExecutor(ctx, step.ChildDAG.Name)
	if err != nil {
		return nil, err
	}

	// Set the worker selector from the step
	child.SetWorkerSelector(step.WorkerSelector)

	dir := GetEnv(ctx).WorkingDir
	if dir != "" && !fileutil.FileExists(dir) {
		return nil, ErrWorkingDirNotExist
	}

	return &dagExecutor{
		child:   child,
		workDir: dir,
		step:    step,
	}, nil
}

func (e *dagExecutor) Run(ctx context.Context) error {
	// Ensure cleanup happens even if there's an error
	defer func() {
		if err := e.child.Cleanup(ctx); err != nil {
			logger.Error(ctx, "Failed to cleanup child DAG executor", "error", err)
		}
	}()

	// Check if we should use distributed execution
	if e.child.ShouldUseDistributedExecution() {
		logger.Info(ctx, "Worker selector specified for child DAG execution",
			"dag", e.child.DAG.Name,
			"workerSelector", e.step.WorkerSelector,
		)

		// Try distributed execution
		err := e.runDistributed(ctx)
		if err != nil {
			// Log the error but fall through to local execution
			logger.Warn(ctx, "Distributed execution failed, falling back to local execution",
				"error", err,
				"dag", e.child.DAG.Name,
			)
		} else {
			// Distributed execution succeeded
			return nil
		}
	}

	e.lock.Lock()

	cmd, err := e.child.BuildCommand(ctx, e.runParams, e.workDir)
	if err != nil {
		e.lock.Unlock()
		return err
	}

	if e.stdout != nil {
		cmd.Stdout = e.stdout
	}
	if e.stderr != nil {
		cmd.Stderr = e.stderr
	}

	e.cmd = cmd

	logger.Info(ctx, "Executing child DAG",
		"dagRunId", e.runParams.RunID,
		"target", e.child.DAG.Name,
	)

	err = cmd.Start()
	e.lock.Unlock()

	if err != nil {
		return fmt.Errorf("failed to start child dag-run: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("child dag-run failed: %w", err)
	}

	// get results from the child dag-run
	env := GetEnv(ctx)
	result, err := env.DB.GetChildDAGRunStatus(ctx, e.runParams.RunID, env.RootDAGRun)
	if err != nil {
		return fmt.Errorf("failed to find result for the child dag-run %q: %w", e.runParams.RunID, err)
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	// add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	if e.stdout != nil {
		if _, err := e.stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return nil
}

func (e *dagExecutor) SetParams(params RunParams) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.runParams = params
}

func (e *dagExecutor) SetStdout(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stdout = out
}

func (e *dagExecutor) SetStderr(out io.Writer) {
	e.lock.Lock()
	defer e.lock.Unlock()
	e.stderr = out
}

func (e *dagExecutor) Kill(sig os.Signal) error {
	e.lock.Lock()
	defer e.lock.Unlock()
	return killProcessGroup(e.cmd, sig)
}

// runDistributed attempts to execute the child DAG via the coordinator
func (e *dagExecutor) runDistributed(ctx context.Context) error {
	// Build the coordinator task
	task, err := e.child.BuildCoordinatorTask(ctx, e.runParams)
	if err != nil {
		return fmt.Errorf("failed to build coordinator task: %w", err)
	}

	// Create coordinator client
	coordinatorClient, err := e.getCoordinatorClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create coordinator client: %w", err)
	}
	defer coordinatorClient.Close()

	// Dispatch the task
	logger.Info(ctx, "Dispatching task to coordinator",
		"dag_run_id", task.DagRunId,
		"target", task.Target,
		"worker_selector", task.WorkerSelector,
	)

	if err := coordinatorClient.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	// Wait for distributed execution to complete
	err = e.waitForDistributedExecution(ctx, e.runParams.RunID)
	if err != nil {
		return fmt.Errorf("distributed execution failed: %w", err)
	}

	return nil
}

// getCoordinatorClient creates a coordinator client using system configuration
func (e *dagExecutor) getCoordinatorClient(ctx context.Context) (client.Client, error) {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	// Use worker configuration for coordinator connection
	// Workers connect to the coordinator, so we use the worker's coordinator settings
	host := cfg.Worker.CoordinatorHost
	port := cfg.Worker.CoordinatorPort

	// Default values if not configured
	if host == "" {
		host = "localhost"
	}
	if port == 0 {
		port = 8084
	}

	// Create client factory
	factory := client.NewFactory().
		WithHost(host).
		WithPort(port)

	// Configure TLS if provided
	if cfg.Worker.TLS != nil && cfg.Worker.TLS.CertFile != "" {
		factory.WithTLS(
			cfg.Worker.TLS.CertFile,
			cfg.Worker.TLS.KeyFile,
			cfg.Worker.TLS.CAFile,
		)
		if cfg.Worker.SkipTLSVerify {
			factory.WithSkipTLSVerify(true)
		}
	} else if cfg.Worker.Insecure {
		// Explicitly insecure connection
		factory.WithInsecure()
	} else {
		// Default to insecure if not specified
		factory.WithInsecure()
	}

	// Build and return the client
	return factory.Build(ctx)
}

// waitForDistributedExecution polls for the completion of a distributed task
func (e *dagExecutor) waitForDistributedExecution(ctx context.Context, dagRunID string) error {
	env := GetEnv(ctx)

	// Poll for completion
	pollInterval := 5 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("distributed execution cancelled: %w", ctx.Err())
		case <-ticker.C:
			// Check if the child DAG run has completed
			result, err := env.DB.GetChildDAGRunStatus(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				// Not found yet, continue polling
				logger.Debug(ctx, "Child DAG run status not available yet",
					"dag_run_id", dagRunID,
					"error", err,
				)
				continue
			}

			// If we got a result, the child DAG has completed
			logger.Info(ctx, "Distributed execution completed",
				"dag_run_id", dagRunID,
				"name", result.Name,
				"is_success", result.Success,
			)

			// Write the results to stdout if available
			if e.stdout != nil {
				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal outputs: %w", err)
				}
				jsonData = append(jsonData, '\n')

				if _, err := e.stdout.Write(jsonData); err != nil {
					return fmt.Errorf("failed to write outputs: %w", err)
				}
			}

			// Check if the execution was successful
			if !result.Success {
				return fmt.Errorf("child DAG execution failed")
			}

			return nil
		}
	}
}

func init() {
	Register(digraph.ExecutorTypeDAGLegacy, newDAGExecutor)
	Register(digraph.ExecutorTypeDAG, newDAGExecutor)
}
