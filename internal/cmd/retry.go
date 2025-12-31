package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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

var retryFlags = []commandLineFlag{dagRunIDFlagRetry, stepNameForRetry, retryWorkerIDFlag}

// retryWorkerIDFlag identifies which worker executes this DAG run retry (for distributed execution tracking)
var retryWorkerIDFlag = commandLineFlag{
	name:  "worker-id",
	usage: "Worker ID executing this DAG run (auto-set in distributed mode, defaults to 'local')",
}

func runRetry(ctx *Context, args []string) error {
	// Extract retry details
	dagRunID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")

	// Get worker-id for tracking which worker executes this DAG run
	workerID, _ := ctx.StringParam("worker-id")
	// Default to "local" for non-distributed execution
	if workerID == "" {
		workerID = "local"
	}

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	// Retrieve the previous run data for specified dag-run ID.
	ref := execution.NewDAGRunRef(name, dagRunID)
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

	// Set DAG context for all logs
	ctx.Context = logger.WithValues(ctx.Context, tag.DAG(dag.Name), tag.RunID(dagRunID))

	// Try lock proc store to avoid race
	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
		return fmt.Errorf("failed to lock process group: %w", err)
	}

	// Acquire process handle
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

	// Unlock the process group before start DAG
	ctx.ProcStore.Unlock(ctx, dag.ProcGroup())

	// The retry command is currently only supported for root DAGs.
	if err := executeRetry(ctx, dag, status, status.DAGRun(), stepName, workerID); err != nil {
		return fmt.Errorf("failed to execute retry: %w", err)
	}

	return nil
}

// executeRetry prepares and runs a retry of a DAG run by opening the original run's log,
// loading the DAG environment, initializing the DAG store, creating an agent configured
// for retry (including step-level retry if provided), and invoking the shared agent executor.
// It returns an error if any setup step fails or if the agent execution fails.
func executeRetry(ctx *Context, dag *core.DAG, status *execution.DAGRunStatus, rootRun execution.DAGRunRef, stepName string, workerID string) error {
	// Set step context if specified
	if stepName != "" {
		ctx.Context = logger.WithValues(ctx.Context, tag.Step(stepName))
	}
	logger.Debug(ctx, "Executing dag-run retry")

	// We use the same log file for the retry as the original run.
	logFile, err := fileutil.OpenOrCreateFile(status.Log)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	logger.Info(ctx, "Dag-run retry initiated", tag.File(logFile.Name()))

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
