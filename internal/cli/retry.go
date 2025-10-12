package cli

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/runtime/agent"
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

var retryFlags = []commandLineFlag{dagRunIDFlagRetry, stepNameForRetry, disableMaxActiveRuns}

func runRetry(ctx *Context, args []string) error {
	dagRunID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")
	disableMaxActiveRuns := ctx.Command.Flags().Changed("disable-max-active-runs")

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	// Retrieve the previous run data for specified dag-run ID.
	ref := core.NewDAGRunRef(name, dagRunID)
	attempt, err := ctx.DAGRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
	}

	// Read the detailed status of the previous status.
	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	// Get the DAG instance from the execution history.
	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from record: %w", err)
	}

	// Try lock proc store to avoid race
	if err := ctx.ProcStore.TryLock(ctx, dag.ProcGroup()); err != nil {
		return fmt.Errorf("failed to lock process group: %w", err)
	}
	defer ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	if !disableMaxActiveRuns {
		liveCount, err := ctx.ProcStore.CountAliveByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return fmt.Errorf("failed to access proc store: %w", err)
		}
		// Count queued DAG-runs and return error if the total of a new run plus
		// active runs will exceed the maxActiveRuns.
		queuedRuns, err := ctx.QueueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
		if err != nil {
			return fmt.Errorf("failed to read queue: %w", err)
		}
		// If the DAG has a queue configured and maxActiveRuns > 0, ensure the number
		// of active runs in the queue does not exceed this limit.
		if dag.MaxActiveRuns > 0 && len(queuedRuns)+liveCount >= dag.MaxActiveRuns {
			return fmt.Errorf("DAG %s is already in the queue (maxActiveRuns=%d), cannot start", dag.Name, dag.MaxActiveRuns)
		}
	}

	// Acquire process handle
	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), core.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		logger.Debug(ctx, "failed to acquire process handle", "err", err)
		return fmt.Errorf("failed to acquire process handle: %w", errMaxRunReached)
	}
	defer func() {
		_ = proc.Stop(ctx)
	}()

	// Unlock the process group before start DAG
	ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	// The retry command is currently only supported for root DAGs.
	if err := executeRetry(ctx, dag, status, status.DAGRun(), stepName); err != nil {
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

func executeRetry(ctx *Context, dag *core.DAG, status *models.DAGRunStatus, rootRun core.DAGRunRef, stepName string) error {
	logger.Debug(ctx, "Executing dag-run retry", "dag", dag.Name, "runId", status.DAGRunID, "step", stepName)

	// We use the same log file for the retry as the original run.
	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "dag-run retry initiated", "DAG", dag.Name, "dagRunId", status.DAGRunID, "logFile", logFile.Name(), "step", stepName)

	// Load environment variable
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
		ctx.Config.Global.Peer,
		agent.Options{
			RetryTarget:     status,
			ParentDAGRun:    status.Parent,
			ProgressDisplay: shouldEnableProgress(ctx),
			StepRetry:       stepName,
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, dag, status.DAGRunID, logFile)
}
