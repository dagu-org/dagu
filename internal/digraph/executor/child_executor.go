package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/coordinator/client"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/otel"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// ChildDAGExecutor is a helper for executing child DAGs.
// It handles both regular DAGs and local DAGs (defined in the same file).
type ChildDAGExecutor struct {
	// DAG is the child DAG to execute.
	// For local DAGs, this DAG's Location will be set to a temporary file.
	DAG *digraph.DAG

	// tempFile holds the temporary file path for local DAGs.
	// This will be cleaned up after execution.
	tempFile string

	// coordinatorClientFactory is used for distributed execution
	coordinatorClientFactory *client.Factory

	// Process tracking for local execution
	mu  sync.Mutex
	cmd *exec.Cmd
}

// NewChildDAGExecutor creates a new ChildDAGExecutor.
// It handles the logic for finding the DAG - either from the database
// or from local DAGs defined in the parent.
func NewChildDAGExecutor(ctx context.Context, childName string) (*ChildDAGExecutor, error) {
	env := GetEnv(ctx)

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
				DAG:                      &dag,
				tempFile:                 tempFile,
				coordinatorClientFactory: env.CoordinatorClientFactory,
			}, nil
		}
	}

	// If not found as local DAG, look it up in the database
	dag, err := env.DB.GetDAG(ctx, childName)
	if err != nil {
		return nil, fmt.Errorf("failed to find DAG %q: %w", childName, err)
	}

	return &ChildDAGExecutor{
		DAG:                      dag,
		coordinatorClientFactory: env.CoordinatorClientFactory,
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

	env := GetEnv(ctx)
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

	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config", configFile)
		}
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

	setupCommand(cmd)

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
	env := GetEnv(ctx)

	if runParams.RunID == "" {
		return nil, fmt.Errorf("dag-run ID is not set")
	}

	if env.RootDAGRun.Zero() {
		return nil, fmt.Errorf("root dag-run ID is not set")
	}

	// Build task for coordinator dispatch using DAG.CreateTask
	task := e.DAG.CreateTask(
		coordinatorv1.Operation_OPERATION_START,
		runParams.RunID,
		digraph.WithRootDagRun(env.RootDAGRun),
		digraph.WithParentDagRun(digraph.DAGRunRef{
			Name: env.DAG.Name,
			ID:   env.DAGRunID,
		}),
		digraph.WithTaskParams(runParams.Params),
		digraph.WithWorkerSelector(e.DAG.WorkerSelector),
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

// Execute executes the child DAG either locally or via coordinator based on configuration.
// It writes the output to the provided stdout writer.
func (e *ChildDAGExecutor) Execute(ctx context.Context, runParams RunParams, workDir string, stdout io.Writer) error {
	// Check if we should use distributed execution
	if e.ShouldUseDistributedExecution() {
		return e.executeDistributed(ctx, runParams, stdout)
	}

	// Local execution
	return e.executeLocal(ctx, runParams, workDir, stdout)
}

// ExecuteWithResult executes the child DAG and returns the result.
// This is useful for parallel execution where results need to be collected.
func (e *ChildDAGExecutor) ExecuteWithResult(ctx context.Context, runParams RunParams, workDir string) (*ChildResult, error) {
	// Check if we should use distributed execution
	if e.ShouldUseDistributedExecution() {
		return e.executeDistributedWithResult(ctx, runParams)
	}

	// Local execution
	cmd, err := e.buildCommand(ctx, runParams, workDir)
	if err != nil {
		return nil, err
	}

	// Create pipes for stdout/stderr capture
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	// Store command reference for Kill
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	// Ensure we clear command reference when done
	defer func() {
		e.mu.Lock()
		e.cmd = nil
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

	err = cmd.Wait()

	// Check if the error is due to context cancellation
	if ctx.Err() == context.Canceled {
		return nil, fmt.Errorf("child dag execution cancelled")
	}

	// Get the result regardless of error
	env := GetEnv(ctx)
	result, resultErr := env.DB.GetChildDAGRunStatus(ctx, runParams.RunID, env.RootDAGRun)

	// Build child result
	if resultErr == nil && result != nil {
		// Convert digraph.Status outputs to map[string]any
		outputs := make(map[string]any)
		for k, v := range result.Outputs {
			outputs[k] = v
		}

		// Determine success based on execution error
		success := err == nil

		return &ChildResult{
			RunID:    runParams.RunID,
			Params:   runParams.Params,
			Success:  success,
			Output:   outputs,
			ExitCode: cmd.ProcessState.ExitCode(),
		}, nil
	}

	// Even if we couldn't get the result, return what we know
	success := err == nil
	exitCode := -1
	if cmd.ProcessState != nil {
		exitCode = cmd.ProcessState.ExitCode()
	}

	return &ChildResult{
		RunID:    runParams.RunID,
		Params:   runParams.Params,
		Success:  success,
		Error:    fmt.Sprintf("execution error: %v, result error: %v", err, resultErr),
		ExitCode: exitCode,
	}, nil
}

// executeLocal runs the child DAG locally
func (e *ChildDAGExecutor) executeLocal(ctx context.Context, runParams RunParams, workDir string, stdout io.Writer) error {
	cmd, err := e.buildCommand(ctx, runParams, workDir)
	if err != nil {
		return err
	}

	if stdout != nil {
		cmd.Stdout = stdout
	}

	// Store command reference for Kill
	e.mu.Lock()
	e.cmd = cmd
	e.mu.Unlock()

	// Ensure we clear command reference when done
	defer func() {
		e.mu.Lock()
		e.cmd = nil
		e.mu.Unlock()
	}()

	logger.Info(ctx, "Executing child DAG locally",
		"dagRunId", runParams.RunID,
		"target", e.DAG.Name,
	)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start child dag-run: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		// Check if the error is due to context cancellation
		if ctx.Err() == context.Canceled {
			return fmt.Errorf("child dag execution cancelled")
		}
		return fmt.Errorf("child dag-run failed: %w", err)
	}

	// Get results from the child dag-run
	env := GetEnv(ctx)
	result, err := env.DB.GetChildDAGRunStatus(ctx, runParams.RunID, env.RootDAGRun)
	if err != nil {
		return fmt.Errorf("failed to find result for the child dag-run %q: %w", runParams.RunID, err)
	}

	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outputs: %w", err)
	}

	// Add a newline at the end of the JSON output
	jsonData = append(jsonData, '\n')

	if stdout != nil {
		if _, err := stdout.Write(jsonData); err != nil {
			return fmt.Errorf("failed to write outputs: %w", err)
		}
	}

	return nil
}

// executeDistributed runs the child DAG via coordinator
func (e *ChildDAGExecutor) executeDistributed(ctx context.Context, runParams RunParams, stdout io.Writer) error {
	// Dispatch to coordinator
	if err := e.dispatchToCoordinator(ctx, runParams); err != nil {
		return fmt.Errorf("distributed execution failed: %w", err)
	}

	// Wait for completion
	return e.waitForCompletion(ctx, runParams.RunID, stdout)
}

// executeDistributedWithResult runs the child DAG via coordinator and returns the result
func (e *ChildDAGExecutor) executeDistributedWithResult(ctx context.Context, runParams RunParams) (*ChildResult, error) {
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

	// Create coordinator client
	coordinatorClient, err := e.getCoordinatorClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create coordinator client: %w", err)
	}
	defer func() {
		if err := coordinatorClient.Close(); err != nil {
			logger.Error(ctx, "Failed to close coordinator client", "err", err)
		}
	}()

	// Dispatch the task
	logger.Info(ctx, "Dispatching task to coordinator",
		"dag_run_id", task.DagRunId,
		"target", task.Target,
		"worker_selector", task.WorkerSelector,
	)

	if err := coordinatorClient.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	return nil
}

// waitForCompletion polls for the completion of a distributed task
func (e *ChildDAGExecutor) waitForCompletion(ctx context.Context, dagRunID string, stdout io.Writer) error {
	env := GetEnv(ctx)

	// Poll for completion
	pollInterval := 1 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("distributed execution cancelled: %w", ctx.Err())
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
				"is_success", result.Success,
			)

			// Write the results to stdout if available
			if stdout != nil {
				jsonData, err := json.MarshalIndent(result, "", "  ")
				if err != nil {
					return fmt.Errorf("failed to marshal outputs: %w", err)
				}
				jsonData = append(jsonData, '\n')

				if _, err := stdout.Write(jsonData); err != nil {
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

// waitForCompletionWithResult is similar to waitForCompletion but returns the result
func (e *ChildDAGExecutor) waitForCompletionWithResult(ctx context.Context, dagRunID string) (*ChildResult, error) {
	env := GetEnv(ctx)

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
				"is_success", result.Success,
			)

			// Convert to ChildResult
			childResult := &ChildResult{
				RunID:    dagRunID,
				Params:   result.Params,
				Success:  result.Success,
				Output:   convertOutputsToMap(result.Outputs),
				Error:    getErrorString(result),
				ExitCode: getExitCode(result.Success),
			}

			return childResult, nil
		}
	}
}

// getCoordinatorClient gets a coordinator client using the factory from environment
func (e *ChildDAGExecutor) getCoordinatorClient(ctx context.Context) (client.Client, error) {
	// Factory should be initialized when Env is created
	if e.coordinatorClientFactory == nil {
		return nil, fmt.Errorf("coordinator client factory not initialized in environment")
	}

	// Build client from factory
	return e.coordinatorClientFactory.Build(ctx)
}

func getErrorString(result *digraph.Status) string {
	if !result.Success {
		return "child DAG execution failed"
	}
	return ""
}

func getExitCode(success bool) int {
	if success {
		return 0
	}
	return 1
}

// Kill terminates the running child DAG process
func (e *ChildDAGExecutor) Kill(sig os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil {
		logger.Info(context.Background(), "Killing child DAG process",
			"dag", e.DAG.Name,
			"pid", e.cmd.Process.Pid,
			"signal", sig,
		)
		return killProcessGroup(e.cmd, sig)
	}

	return nil
}

// convertOutputsToMap converts string map to map[string]any
func convertOutputsToMap(outputs map[string]string) map[string]any {
	if outputs == nil {
		return nil
	}
	result := make(map[string]any)
	for k, v := range outputs {
		result[k] = v
	}
	return result
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
	return otel.InjectTraceContext(ctx)
}
