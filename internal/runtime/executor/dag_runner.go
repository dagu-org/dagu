package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/telemetry"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

var (
	errSubDAGCancelled  = errors.New("sub DAG execution cancelled")
	errDAGRunIDNotSet   = errors.New("DAG run ID is not set")
	errRootDAGRunNotSet = errors.New("root DAG run ID is not set")
)

// SubDAGExecutor is a helper for executing sub DAGs.
// It handles both regular DAGs and local DAGs (defined in the same file).
type SubDAGExecutor struct {
	// DAG is the sub DAG to execute.
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
	dagCtx          execution.Context    // for DB access when cancelling distributed runs

	// killed should be closed when Kill is called
	killed     chan struct{}
	cancelOnce sync.Once
}

// NewSubDAGExecutor creates a new SubDAGExecutor.
// It handles the logic for finding the DAG - either from the database
// or from local DAGs defined in the parent.
func NewSubDAGExecutor(ctx context.Context, childName string) (*SubDAGExecutor, error) {
	rCtx := execution.GetContext(ctx)

	// First, check if it's a local DAG in the parent
	if rCtx.DAG != nil && rCtx.DAG.LocalDAGs != nil {
		if localDAG, ok := rCtx.DAG.LocalDAGs[childName]; ok {
			// Create a temporary file for the local DAG
			tempFile, err := createTempDAGFile(childName, localDAG.YamlData)
			if err != nil {
				return nil, fmt.Errorf("failed to create temp file for local DAG: %w", err)
			}

			// Set the location to the temporary file
			dag := *localDAG // copy the DAG to avoid modifying the original
			dag.Location = tempFile

			return &SubDAGExecutor{
				DAG:             &dag,
				tempFile:        tempFile,
				coordinatorCli:  rCtx.CoordinatorCli,
				cmds:            make(map[string]*exec.Cmd),
				distributedRuns: make(map[string]bool),
				dagCtx:          rCtx,
				killed:          make(chan struct{}),
			}, nil
		}
	}

	// If not found as local DAG, look it up in the database
	dag, err := rCtx.DB.GetDAG(ctx, childName)
	if err != nil {
		return nil, fmt.Errorf("failed to find DAG %q: %w", childName, err)
	}

	return &SubDAGExecutor{
		DAG:             dag,
		coordinatorCli:  rCtx.CoordinatorCli,
		cmds:            make(map[string]*exec.Cmd),
		distributedRuns: make(map[string]bool),
		dagCtx:          rCtx,
		killed:          make(chan struct{}),
	}, nil
}

// buildCommand builds the command to execute the sub DAG.
func (e *SubDAGExecutor) buildCommand(ctx context.Context, runParams RunParams, workDir string) (*exec.Cmd, error) {
	executable, err := executablePath()
	if err != nil {
		return nil, fmt.Errorf("failed to find executable path: %w", err)
	}

	if runParams.RunID == "" {
		return nil, errDAGRunIDNotSet
	}

	rCtx := execution.GetContext(ctx)
	if rCtx.RootDAGRun.Zero() {
		return nil, errRootDAGRunNotSet
	}

	args := []string{
		"start",
		fmt.Sprintf("--root=%s", rCtx.RootDAGRun.String()),
		fmt.Sprintf("--parent=%s", rCtx.DAGRunRef().String()),
		fmt.Sprintf("--run-id=%s", runParams.RunID),
		"--no-queue",
		"--disable-max-active-runs",
	}
	if workDir != "" {
		args = append(args, fmt.Sprintf("--default-working-dir=%s", workDir))
	}
	if configFile := config.ConfigFileUsed(ctx); configFile != "" {
		args = append(args, "--config", configFile)
	}
	args = append(args, e.DAG.Location)

	if runParams.Params != "" {
		args = append(args, "--", runParams.Params)
	}

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = workDir
	cmd.Env = append(cmd.Env, rCtx.AllEnvs()...)

	// Inject OpenTelemetry trace context into environment variables
	logCtx := logger.WithValues(ctx, tag.DAG(e.DAG.Name))
	traceEnvVars := extractTraceContext(ctx)
	if len(traceEnvVars) > 0 {
		cmd.Env = append(cmd.Env, traceEnvVars...)
		logger.Debug(logCtx, "Injecting trace context into sub DAG",
			slog.Any("trace-env-vars", traceEnvVars),
		)
	} else {
		logger.Debug(logCtx, "No trace context to inject into sub DAG")
	}

	cmdutil.SetupCommand(cmd)
	return cmd, nil
}

// BuildCoordinatorTask creates a coordinator task for distributed execution
func (e *SubDAGExecutor) BuildCoordinatorTask(ctx context.Context, runParams RunParams) (*coordinatorv1.Task, error) {
	rCtx := execution.GetContext(ctx)

	if runParams.RunID == "" {
		return nil, fmt.Errorf("dag-run ID is not set")
	}

	if rCtx.RootDAGRun.Zero() {
		return nil, fmt.Errorf("root dag-run ID is not set")
	}

	// Build task for coordinator dispatch using DAG.CreateTask
	task := CreateTask(
		e.DAG.Name,
		string(e.DAG.YamlData),
		coordinatorv1.Operation_OPERATION_START,
		runParams.RunID,
		WithRootDagRun(rCtx.RootDAGRun),
		WithParentDagRun(execution.DAGRunRef{
			Name: rCtx.DAG.Name,
			ID:   rCtx.DAGRunID,
		}),
		WithTaskParams(runParams.Params),
		WithWorkerSelector(e.DAG.WorkerSelector),
	)

	taskCtx := logger.WithValues(ctx,
		tag.RunID(runParams.RunID),
		tag.Target(e.DAG.Name),
	)
	logger.Info(taskCtx, "Built coordinator task for sub DAG",
		slog.Any("worker-selector", e.DAG.WorkerSelector),
	)

	return task, nil
}

// Cleanup removes any temporary files created for local DAGs.
// This should be called after the sub DAG execution is complete.
func (e *SubDAGExecutor) Cleanup(ctx context.Context) error {
	if e.tempFile == "" {
		return nil
	}

	ctx = logger.WithValues(ctx, tag.File(e.tempFile))
	logger.Info(ctx, "Cleaning up temporary DAG file")

	if err := os.Remove(e.tempFile); err != nil && !os.IsNotExist(err) {
		logger.Error(ctx, "Failed to remove temporary DAG file", tag.File(e.tempFile), tag.Error(err))
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}

// Execute executes the sub DAG and returns the result.
// This is useful for parallel execution where results need to be collected.
func (e *SubDAGExecutor) Execute(ctx context.Context, runParams RunParams, workDir string) (*execution.RunStatus, error) {
	ctx = logger.WithValues(ctx, tag.SubDAG(e.DAG.Name), tag.SubRunID(runParams.RunID))

	if len(e.DAG.WorkerSelector) > 0 {
		// Handle distributed execution
		logger.Info(ctx, "Executing sub DAG via distributed execution")

		e.mu.Lock()
		e.distributedRuns[runParams.RunID] = true
		e.mu.Unlock()

		return e.dispatch(ctx, runParams)
	}

	// Handle local execution
	cmd, waitErr := e.buildCommand(ctx, runParams, workDir)
	if waitErr != nil {
		return nil, waitErr
	}

	// Create pipes for stdout/stderr capture
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	// Ensure we clear command reference when done
	defer func() {
		e.mu.Lock()
		delete(e.cmds, runParams.RunID)
		e.mu.Unlock()
	}()

	logger.Info(ctx, "Executing sub DAG locally")

	// Start the command first to initialize cmd.Process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start sub dag-run: %w", err)
	}

	// Store command reference for Kill AFTER starting, so cmd.Process is already set
	e.mu.Lock()
	e.cmds[runParams.RunID] = cmd
	e.mu.Unlock()

	waitErr = cmd.Wait()
	if waitErr != nil {
		logger.Error(ctx, "Sub DAG execution returned error", tag.Error(waitErr))
	}

	select {
	case <-e.killed:
		return nil, errSubDAGCancelled
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %w", errSubDAGCancelled, ctx.Err())
	default:
	}

	rCtx := execution.GetContext(ctx)
	result, resultErr := rCtx.DB.GetSubDAGRunStatus(ctx, runParams.RunID, rCtx.RootDAGRun)
	if resultErr != nil {
		return nil, fmt.Errorf("failed to find result for the sub dag-run %q: %w", runParams.RunID, resultErr)
	}

	if result.Status.IsSuccess() {
		logger.Info(ctx, "Sub DAG completed successfully")
		return result, nil
	}

	return result, waitErr
}

// executeDistributedWithResult runs the sub DAG via coordinator and returns the result
func (e *SubDAGExecutor) dispatch(ctx context.Context, runParams RunParams) (*execution.RunStatus, error) {
	distCtx := logger.WithValues(ctx,
		tag.RunID(runParams.RunID),
		tag.DAG(e.DAG.Name),
	)

	// Dispatch to coordinator
	err := e.dispatchToCoordinator(ctx, runParams)

	if ctx.Err() != nil {
		logger.Info(distCtx, "Cancellation requested for distributed sub DAG dispatch")
		return nil, ctx.Err()
	}

	if err != nil {
		logger.Error(distCtx, "Distributed sub DAG dispatch failed", tag.Error(err))
		return nil, fmt.Errorf("distributed execution failed: %w", err)
	}

	logger.Info(distCtx, "Distributed sub DAG dispatched; awaiting completion")
	return e.waitCompletion(ctx, runParams.RunID)
}

// dispatchToCoordinator builds and dispatches a task to the coordinator
func (e *SubDAGExecutor) dispatchToCoordinator(ctx context.Context, runParams RunParams) error {
	// Build the coordinator task
	task, err := e.BuildCoordinatorTask(ctx, runParams)
	if err != nil {
		return fmt.Errorf("failed to build coordinator task: %w", err)
	}

	if e.coordinatorCli == nil {
		return fmt.Errorf("no coordinator client configured for distributed execution")
	}

	// Dispatch the task
	dispatchCtx := logger.WithValues(ctx,
		tag.RunID(task.DagRunId),
		tag.Target(task.Target),
	)
	logger.Info(dispatchCtx, "Dispatching task to coordinator",
		slog.Any("worker-selector", task.WorkerSelector),
	)

	if err := e.coordinatorCli.Dispatch(ctx, task); err != nil {
		return fmt.Errorf("failed to dispatch task: %w", err)
	}

	return nil
}

func (e *SubDAGExecutor) waitCompletion(ctx context.Context, dagRunID string) (*execution.RunStatus, error) {
	rCtx := execution.GetContext(ctx)
	waitCtx := logger.WithValues(ctx,
		tag.RunID(dagRunID),
		tag.DAG(e.DAG.Name),
	)

	// Poll for completion
	pollInterval := 1 * time.Second
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	// Periodically log while waiting so long polls do not look stalled
	logInterval := 15 * time.Second
	logTicker := time.NewTicker(logInterval)
	defer logTicker.Stop()
	start := time.Now()

	for {
		select {
		case <-e.killed:
			logger.Info(waitCtx, "Cancellation requested for distributed sub DAG run; waiting for termination")

			// Timeout to allow cancellation to propagate
			timeout := time.After(30 * time.Second)

			// Wait for sub DAGs to be finished being killed
			killTicker := time.NewTicker(1 * time.Second)
			defer killTicker.Stop()

			killLogTicker := time.NewTicker(5 * time.Second)
			defer killLogTicker.Stop()

			var status *execution.RunStatus

			for {
				var err error
				status, err = rCtx.DB.GetSubDAGRunStatus(ctx, dagRunID, rCtx.RootDAGRun)
				if err != nil {
					logger.Warn(waitCtx, "Failed to get sub DAG run status during cancellation wait",
						tag.Error(err),
					)
				}
				if status != nil && !status.Status.IsActive() {
					return status, nil
				}

				select {
				case <-timeout:
					return nil, fmt.Errorf("distributed execution cancellation timed out for dag-run ID %s", dagRunID)

				case <-killTicker.C:
					// continue waiting

				case <-killLogTicker.C:
					lastStatus := "unknown"
					if status != nil {
						lastStatus = status.Status.String()
					}

					logger.Info(waitCtx, "Still waiting for distributed sub DAG run to terminate",
						tag.Duration(time.Since(start).Round(time.Second)),
						tag.Status(lastStatus),
					)
				}
			}

		case <-ticker.C:
			// Check if the sub DAG run has completed
			isCompleted, err := rCtx.DB.IsSubDAGRunCompleted(ctx, dagRunID, rCtx.RootDAGRun)
			if err != nil {
				logger.Warn(waitCtx, "Failed to check sub DAG run completion",
					tag.Error(err),
				)
				continue // Retry on error
			}

			if !isCompleted {
				logger.Debug(waitCtx, "Sub DAG run not completed yet")
				continue // Not completed, keep polling
			}

			// Check the final status of the sub DAG run
			result, err := rCtx.DB.GetSubDAGRunStatus(ctx, dagRunID, rCtx.RootDAGRun)
			if err != nil {
				// Not found yet, continue polling
				logger.Debug(waitCtx, "Sub DAG run status not available yet",
					tag.Error(err),
				)
				continue
			}

			// If we got a result, the sub DAG has completed
			logger.Info(waitCtx, "Distributed execution completed",
				tag.Name(result.Name),
			)

			return result, nil

		case <-logTicker.C:
			logger.Info(waitCtx, "Waiting for distributed sub DAG run to complete",
				tag.Duration(time.Since(start).Round(time.Second)),
			)
		}
	}
}

// Kill terminates all running sub DAG processes (both local and distributed)
func (e *SubDAGExecutor) Kill(sig os.Signal) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	var errs []error

	// Cancel distributed runs
	ctx := context.Background()
	for runID := range e.distributedRuns {
		if err := e.dagCtx.DB.RequestChildCancel(ctx, runID, e.dagCtx.RootDAGRun); err != nil {
			if errors.Is(err, execution.ErrDAGRunIDNotFound) {
				continue
			}
			errs = append(errs, err)
			logger.Warn(ctx, "Failed to request child cancel",
				tag.RunID(runID),
				tag.DAG(e.DAG.Name),
				tag.Error(err),
			)
		} else {
			logger.Info(ctx, "Requested distributed sub DAG cancellation",
				tag.RunID(runID),
				tag.DAG(e.DAG.Name),
			)
		}
	}

	// Kill local processes
	for runID, cmd := range e.cmds {
		if cmd != nil && cmd.Process != nil {
			if err := cmdutil.KillProcessGroup(cmd, sig); err != nil {
				errs = append(errs, err)
				logger.Warn(ctx, "Failed to kill local sub DAG process",
					tag.RunID(runID),
					tag.DAG(e.DAG.Name),
					tag.Error(err),
				)
			} else {
				logger.Info(ctx, "Requested kill for local sub DAG process",
					tag.RunID(runID),
					tag.DAG(e.DAG.Name),
				)
			}
		}
	}

	e.cancelOnce.Do(func() {
		close(e.killed)
	})

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
