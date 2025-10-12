package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// ChildDAGExecutor is a helper for executing child DAGs.
// It handles both regular DAGs and local DAGs (defined in the same file).
type ChildDAGExecutor struct {
	// DAG is the child DAG to execute.
	// For local DAGs, this DAG's Location will be set to a temporary file.
	DAG *core.DAG

	// tempFile holds the temporary file path for local DAGs.
	// This will be cleaned up after execution.
	tempFile string

	// coordinatorCli is used for distributed execution
	coordinatorCli execution.Dispatcher

	// Process tracking for ALL executions
	mu              sync.Mutex
	cmds            map[string]*exec.Cmd // runID -> cmd for local processes
	distributedRuns map[string]bool      // runID -> true for distributed runs
	env             execution.Env        // for DB access when cancelling distributed runs
}

// NewChildDAGExecutor creates a new ChildDAGExecutor.
// It handles the logic for finding the DAG - either from the database
// or from local DAGs defined in the parent.
func NewChildDAGExecutor(ctx context.Context, childName string) (*ChildDAGExecutor, error) {
	env := execution.GetEnv(ctx)

	// First, check if it's a local DAG in the parent
	if env.DAG != nil && env.DAG.LocalDAGs != nil {
		if localDAG, ok := env.DAG.LocalDAGs[childName]; ok {
			// Create a temporary file for the local DAG
			tempFile, err := createTempDAGFile(childName, localDAG.YamlData)
			if err != nil {
				return nil, fmt.Errorf("failed to create temp file for local DAG: %w", err)
			}

			// Set the location to the temporary file
			dag := *localDAG // copy the DAG to avoid modifying the original
			dag.Location = tempFile

			return &ChildDAGExecutor{
				DAG:             &dag,
				tempFile:        tempFile,
				coordinatorCli:  env.CoordinatorCli,
				cmds:            make(map[string]*exec.Cmd),
				distributedRuns: make(map[string]bool),
				env:             execution.GetEnv(ctx),
			}, nil
		}
	}

	// If not found as local DAG, look it up in the database
	dag, err := env.DB.GetDAG(ctx, childName)
	if err != nil {
		return nil, fmt.Errorf("failed to find DAG %q: %w", childName, err)
	}

	return &ChildDAGExecutor{
		DAG:             dag,
		coordinatorCli:  env.CoordinatorCli,
		cmds:            make(map[string]*exec.Cmd),
		distributedRuns: make(map[string]bool),
		env:             execution.GetEnv(ctx),
	}, nil
}

// buildCommand builds the command to execute the child DAG.
func (e *ChildDAGExecutor) buildCommand(
	ctx context.Context,
	runParams RunParams,
	workDir string,
) (*exec.Cmd, error) {
	executable, err := executablePath()
	if err != nil {
		return nil, fmt.Errorf("failed to find executable path: %w", err)
	}

	if runParams.RunID == "" {
		return nil, fmt.Errorf("dag-run ID is not set")
	}

	env := execution.GetEnv(ctx)
	if env.RootDAGRun.Zero() {
		return nil, fmt.Errorf("root dag-run ID is not set")
	}

	args := []string{
		"start",
		fmt.Sprintf("--root=%s", env.RootDAGRun.String()),
		fmt.Sprintf("--parent=%s", env.DAGRunRef().String()),
		fmt.Sprintf("--run-id=%s", runParams.RunID),
		"--no-queue",
		e.DAG.Location,
	}
	if configFile := config.ConfigFileUsed(ctx); configFile != "" {
		args = append(args, "--config", configFile)
	}

	if runParams.Params != "" {
		args = append(args, "--", runParams.Params)
	}

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = workDir
	cmd.Env = append(cmd.Env, env.AllEnvs()...)

	// Inject OpenTelemetry trace context into environment variables
	traceEnvVars := extractTraceContext(ctx)
	if len(traceEnvVars) > 0 {
		cmd.Env = append(cmd.Env, traceEnvVars...)
		logger.Info(ctx, "Injecting trace context into child DAG",
			"traceEnvVars", traceEnvVars,
			"childDAG", e.DAG.Name,
		)
	} else {
		logger.Warn(ctx, "No trace context to inject into child DAG",
			"childDAG", e.DAG.Name,
		)
	}

	cmdutil.SetupCommand(cmd)

	logger.Info(ctx, "Prepared child DAG command",
		"dagRunId", runParams.RunID,
		"target", e.DAG.Name,
		"args", args,
	)

	return cmd, nil
}

// ShouldUseDistributedExecution checks if this child DAG should be executed via coordinator
func (e *ChildDAGExecutor) ShouldUseDistributedExecution() bool {
	// Only use distributed execution if worker selector is specified
	return len(e.DAG.WorkerSelector) > 0
}

// BuildCoordinatorTask creates a coordinator task for distributed execution
func (e *ChildDAGExecutor) BuildCoordinatorTask(
	ctx context.Context,
	runParams RunParams,
) (*coordinatorv1.Task, error) {
	env := execution.GetEnv(ctx)

	if runParams.RunID == "" {
		return nil, fmt.Errorf("dag-run ID is not set")
	}

	if env.RootDAGRun.Zero() {
		return nil, fmt.Errorf("root dag-run ID is not set")
	}

	// Build task for coordinator dispatch using DAG.CreateTask
	task := CreateTask(
		e.DAG.Name,
		string(e.DAG.YamlData),
		coordinatorv1.Operation_OPERATION_START,
		runParams.RunID,
		WithRootDagRun(env.RootDAGRun),
		WithParentDagRun(core.DAGRunRef{
			Name: env.DAG.Name,
			ID:   env.DAGRunID,
		}),
		WithTaskParams(runParams.Params),
		WithWorkerSelector(e.DAG.WorkerSelector),
	)

	logger.Info(ctx, "Built coordinator task for child DAG",
		"dagRunId", runParams.RunID,
		"target", e.DAG.Name,
		"workerSelector", e.DAG.WorkerSelector,
	)

	return task, nil
}

// Cleanup removes any temporary files created for local DAGs.
// This should be called after the child DAG execution is complete.
func (e *ChildDAGExecutor) Cleanup(ctx context.Context) error {
	if e.tempFile == "" {
		return nil
	}

	logger.Info(ctx, "Cleaning up temporary DAG file",
		"dag", e.DAG.Name,
		"tempFile", e.tempFile,
	)

	if err := os.Remove(e.tempFile); err != nil && !os.IsNotExist(err) {
		logger.Error(ctx, "Failed to remove temporary DAG file",
			"dag", e.DAG.Name,
			"tempFile", e.tempFile,
			"err", err,
		)
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}

// ExecuteWithResult executes the child DAG and returns the result.
// This is useful for parallel execution where results need to be collected.
func (e *ChildDAGExecutor) ExecuteWithResult(ctx context.Context, runParams RunParams, workDir string) (*execution.RunStatus, error) {
	// Check if we should use distributed execution
	if e.ShouldUseDistributedExecution() {
		// Track distributed execution
		e.mu.Lock()
		e.distributedRuns[runParams.RunID] = true
		e.mu.Unlock()

		return e.executeDistributedWithResult(ctx, runParams)
	}

	// Local execution
	cmd, waitErr := e.buildCommand(ctx, runParams, workDir)
	if waitErr != nil {
		return nil, waitErr
	}

	// Create pipes for stdout/stderr capture
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	// Store command reference for Kill
	e.mu.Lock()
	e.cmds[runParams.RunID] = cmd
	e.mu.Unlock()

	// Ensure we clear command reference when done
	defer func() {
		e.mu.Lock()
		delete(e.cmds, runParams.RunID)
		e.mu.Unlock()
	}()

	logger.Info(ctx, "Executing child DAG locally",
		"dagRunId", runParams.RunID,
		"target", e.DAG.Name,
		"params", runParams.Params,
	)

	// Start and wait for the command
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start child dag-run: %w", err)
	}

	waitErr = cmd.Wait()

	// Check if the error is due to context cancellation
	if ctx.Err() == context.Canceled {
		return nil, fmt.Errorf("child dag execution cancelled")
	}

	// Get the result regardless of error
	env := execution.GetEnv(ctx)
	result, resultErr := env.DB.GetChildDAGRunStatus(ctx, runParams.RunID, env.RootDAGRun)
	if resultErr != nil {
		return nil, fmt.Errorf("failed to find result for the child dag-run %q: %w", runParams.RunID, resultErr)
	}

	if result.Status.IsSuccess() {
		if waitErr != nil {
			logger.Warn(ctx, "Child DAG completed with exit code but no error", "err", waitErr, "dagRunId", runParams.RunID, "target", e.DAG.Name)
		} else {
			logger.Info(ctx, "Child DAG completed successfully", "dagRunId", runParams.RunID, "target", e.DAG.Name)
		}
		return result, nil
	}

	// Build child result
	return result, waitErr
}

// executeDistributedWithResult runs the child DAG via coordinator and returns the result
func (e *ChildDAGExecutor) executeDistributedWithResult(ctx context.Context, runParams RunParams) (*execution.RunStatus, error) {
	// Dispatch to coordinator
	if err := e.dispatchToCoordinator(ctx, runParams); err != nil {
		return nil, fmt.Errorf("distributed execution failed: %w", err)
	}

	// Wait for completion with result
	return e.waitForCompletionWithResult(ctx, runParams.RunID)
}

// dispatchToCoordinator builds and dispatches a task to the coordinator
func (e *ChildDAGExecutor) dispatchToCoordinator(ctx context.Context, runParams RunParams) error {
	// Build the coordinator task
	task, err := e.BuildCoordinatorTask(ctx, runParams)
	if err != nil {
		return fmt.Errorf("failed to build coordinator task: %w", err)
	}

	if e.coordinatorCli == nil {
		return fmt.Errorf("no coordinator client configured for distributed execution")
	}

	// Dispatch the task
	logger.Info(ctx, "Dispatching task to coordinator",
		"dag_run_id", task.DagRunId,
		"target", task.Target,
		"worker_selector", task.WorkerSelector,
	)

	if err := e.coordinatorCli.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	return nil
}

// waitForCompletionWithResult is similar to waitForCompletion but returns the result
func (e *ChildDAGExecutor) waitForCompletionWithResult(ctx context.Context, dagRunID string) (*execution.RunStatus, error) {
	env := execution.GetEnv(ctx)

	// Poll for completion
	pollInterval := 1 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("distributed execution cancelled: %w", ctx.Err())
		case <-ticker.C:
			// Check if the child DAG run has completed
			isCompleted, err := env.DB.IsChildDAGRunCompleted(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				logger.Error(ctx, "Failed to check child DAG run completion",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue // Retry on error
			}

			if !isCompleted {
				logger.Debug(ctx, "Child DAG run not completed yet",
					"dag_run_id", dagRunID,
				)
				continue // Not completed, keep polling
			}

			// Check the final status of the child DAG run
			result, err := env.DB.GetChildDAGRunStatus(ctx, dagRunID, env.RootDAGRun)
			if err != nil {
				// Not found yet, continue polling
				logger.Debug(ctx, "Child DAG run status not available yet",
					"dag_run_id", dagRunID,
					"err", err,
				)
				continue
			}

			// If we got a result, the child DAG has completed
			logger.Info(ctx, "Distributed execution completed",
				"dag_run_id", dagRunID,
				"name", result.Name,
			)

			return result, nil
		}
	}
}

// Kill terminates all running child DAG processes (both local and distributed)
func (e *ChildDAGExecutor) Kill(sig os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	ctx := context.Background()
	var errs []error

	// Cancel distributed runs
	for runID := range e.distributedRuns {
		logger.Info(ctx, "Requesting cancellation for distributed child DAG",
			"dag", e.DAG.Name,
			"runId", runID,
		)
		if err := e.env.DB.RequestChildCancel(ctx, runID, e.env.RootDAGRun); err != nil {
			logger.Error(ctx, "Failed to request child DAG cancellation",
				"runId", runID,
				"err", err,
			)
			errs = append(errs, err)
		}
	}

	// Kill local processes
	for runID, cmd := range e.cmds {
		if cmd != nil && cmd.Process != nil {
			logger.Info(ctx, "Killing local child DAG process",
				"dag", e.DAG.Name,
				"runId", runID,
				"pid", cmd.Process.Pid,
				"signal", sig,
			)
			if err := cmdutil.KillProcessGroup(cmd, sig); err != nil {
				logger.Error(ctx, "Failed to kill process",
					"runId", runID,
					"err", err,
				)
				errs = append(errs, err)
			}
		}
	}

	// Return the first error if any occurred
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// createTempDAGFile creates a temporary file with the DAG YAML content.
func createTempDAGFile(dagName string, yamlData []byte) (string, error) {
	// Create a temporary directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "dagu", "local-dags")
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a meaningful name
	pattern := fmt.Sprintf("%s-*.yaml", dagName)
	tempFile, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
	}()

	// Write the YAML data
	if _, err := tempFile.Write(yamlData); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write YAML data: %w", err)
	}

	return tempFile.Name(), nil
}

// executablePath returns the path to the dagu executable.
func executablePath() (string, error) {
	if os.Getenv("DAGU_EXECUTABLE") != "" {
		return os.Getenv("DAGU_EXECUTABLE"), nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return executable, nil
}

// extractTraceContext extracts OpenTelemetry trace context from the current context
// and returns it as environment variables for child processes.
func extractTraceContext(ctx context.Context) []string {
	return telemetry.InjectTraceContext(ctx)
}
