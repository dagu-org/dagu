package cmd

import (
	"fmt"
	"log/slog"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/spf13/cobra"
)

func Retry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "retry [flags] <DAG name or file>",
			Short: "Retry a previously executed DAG-run with the same run ID",
			Long: `Create a new run for a previously executed DAG-run using the same DAG-run ID.

Flags:
  --run-id string (required) Unique identifier of the DAG-run to retry.
  --step string (optional) Retry only the specified step.

Examples:
  dagu retry --run-id=abc123 my_dag
  dagu retry --run-id=abc123 my_dag.yaml
`,
			Args: cobra.ExactArgs(1),
		}, retryFlags, runRetry,
	)
}

var retryFlags = []commandLineFlag{dagRunIDFlagRetry, stepNameForRetry, retryWorkerIDFlag}

// retryWorkerIDFlag identifies which worker executes this DAG run retry (for distributed execution tracking)
var retryWorkerIDFlag = commandLineFlag{
	name:  "worker-id",
	usage: "Worker ID executing this DAG run (auto-set in distributed mode, defaults to 'local')",
}

func runRetry(ctx *Context, args []string) error {
	dagRunID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")
	workerID, _ := ctx.StringParam("worker-id")
	if workerID == "" {
		workerID = "local"
	}

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	ref := execution.NewDAGRunRef(name, dagRunID)
	attempt, err := ctx.DAGRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from record: %w", err)
	}

	if len(dag.WorkerSelector) > 0 {
		return fmt.Errorf("cannot retry DAG %q with workerSelector via CLI; use 'dagu enqueue' for distributed execution", dag.Name)
	}

	ctx.Context = logger.WithValues(ctx.Context, tag.DAG(dag.Name), tag.RunID(dagRunID))

	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
		return fmt.Errorf("failed to lock process group: %w", err)
	}

	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), execution.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		ctx.ProcStore.Unlock(ctx, dag.ProcGroup())
		logger.Debug(ctx, "Failed to acquire process handle", tag.Error(err))
		_ = ctx.RecordEarlyFailure(dag, dagRunID, err)
		return fmt.Errorf("failed to acquire process handle: %w", errProcAcquisitionFailed)
	}
	defer func() {
		_ = proc.Stop(ctx)
	}()

	ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	if err := executeRetry(ctx, dag, status, status.DAGRun(), stepName, workerID); err != nil {
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

// executeRetry prepares and runs a retry of a DAG run using the original run's log file.
func executeRetry(ctx *Context, dag *core.DAG, status *execution.DAGRunStatus, rootRun execution.DAGRunRef, stepName string, workerID string) error {
	if stepName != "" {
		ctx.Context = logger.WithValues(ctx.Context, tag.Step(stepName))
	}
	logger.Debug(ctx, "Executing dag-run retry")

	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "Dag-run retry initiated", tag.File(logFile.Name()))

	dag.LoadDotEnv(ctx)

	dr, err := ctx.dagStore(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	agentInstance := agent.New(
		status.DAGRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		ctx.DAGRunStore,
		ctx.ServiceRegistry,
		rootRun,
		ctx.Config.Core.Peer,
		agent.Options{
			RetryTarget:     status,
			ParentDAGRun:    status.Parent,
			ProgressDisplay: shouldEnableProgress(ctx),
			StepRetry:       stepName,
			WorkerID:        workerID,
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, dag, status.DAGRunID, logFile)
}

// dispatchRetryToCoordinatorAndWait dispatches a retry to coordinator and waits for completion.
func dispatchRetryToCoordinatorAndWait(ctx *Context, dag *core.DAG, dagRunID, stepName string, prevStatus *execution.DAGRunStatus, coordinatorCli coordinator.Client) error {
	// Set up signal-aware context so Ctrl+C cancels the operation
	signalCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	signalAwareCtx := ctx.WithContext(signalCtx)

	// Set up progress display early so user sees feedback immediately
	showProgress := shouldEnableProgress(ctx)
	var progress *RemoteProgressDisplay
	if showProgress {
		progress = NewRemoteProgressDisplay(dag, dagRunID)
		progress.Start()
	}

	defer func() {
		if progress != nil {
			progress.Stop()
			if !ctx.Quiet {
				progress.PrintSummary()
			}
		}
	}()

	logger.Info(ctx, "Dispatching retry for distributed execution",
		slog.Any("worker-selector", dag.WorkerSelector),
	)

	opts := []executor.TaskOption{
		executor.WithWorkerSelector(dag.WorkerSelector),
		executor.WithPreviousStatus(prevStatus),
	}
	if stepName != "" {
		opts = append(opts, executor.WithStep(stepName))
	}

	task := executor.CreateTask(
		dag.Name,
		string(dag.YamlData),
		coordinatorv1.Operation_OPERATION_RETRY,
		dagRunID,
		opts...,
	)

	if err := coordinatorCli.Dispatch(signalAwareCtx, task); err != nil {
		return fmt.Errorf("failed to dispatch retry task: %w", err)
	}

	logger.Info(ctx, "Retry dispatched to coordinator; awaiting completion")
	err := waitForDAGCompletionWithProgress(signalAwareCtx, dag, dagRunID, coordinatorCli, progress)

	// If context was cancelled (e.g., Ctrl+C), request cancellation on coordinator
	if signalCtx.Err() != nil {
		return handleDistributedCancellation(ctx, dag, dagRunID, coordinatorCli, progress, err)
	}

	return err
}
