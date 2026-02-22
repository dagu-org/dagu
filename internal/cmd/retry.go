package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
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

var retryWorkerIDFlag = commandLineFlag{
	name:  "worker-id",
	usage: "Worker ID executing this DAG run (auto-set in distributed mode, defaults to 'local')",
}

func runRetry(ctx *Context, args []string) error {
	dagRunID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")
	workerID := getWorkerID(ctx)

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	ref := exec.NewDAGRunRef(name, dagRunID)
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

	dag, err = restoreDAGFromStatus(ctx.Context, dag, status)
	if err != nil {
		return fmt.Errorf("failed to restore DAG from status: %w", err)
	}

	// Block retry via CLI for DAGs with workerSelector, UNLESS this is a distributed worker execution
	// (indicated by --worker-id being set to something other than "local")
	if len(dag.WorkerSelector) > 0 && workerID == "local" {
		return fmt.Errorf("cannot retry DAG %q with workerSelector via CLI; use 'dagu enqueue' for distributed execution", dag.Name)
	}

	// For DAGs using a global queue: when invoked by the user, enqueue the retry
	// so it respects queue capacity. When status is already Queued, we're being
	// invoked by the queue processor to run the item—execute directly.
	// Step retry is not supported via queue (queue processor does not pass step name).
	queueConfig := ctx.Config.FindQueueConfig(dag.ProcGroup())
	if stepName == "" && queueConfig != nil && status.Status != core.Queued {
		return enqueueRetry(ctx, attempt, dag, status, dagRunID)
	}

	ctx.Context = logger.WithValues(ctx.Context, tag.DAG(dag.Name), tag.RunID(dagRunID))

	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
		return fmt.Errorf("failed to lock process group: %w", err)
	}

	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), exec.NewDAGRunRef(dag.Name, dagRunID))
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

	return executeRetry(ctx, dag, status, status.DAGRun(), stepName, workerID)
}

// enqueueRetry enqueues the retry and persists Queued status via exec.EnqueueRetry.
// Retries respect global queue capacity because the queue processor picks them up
// when capacity is available.
func enqueueRetry(ctx *Context, attempt exec.DAGRunAttempt, dag *core.DAG, status *exec.DAGRunStatus, dagRunID string) error {
	if err := exec.EnqueueRetry(ctx.Context, ctx.QueueStore, attempt, dag, status, dagRunID); err != nil {
		return err
	}
	logger.Info(ctx, "Enqueued retry; will run when queue capacity is available",
		tag.DAG(dag.Name),
		tag.RunID(dagRunID),
	)
	return nil
}

// executeRetry runs a retry of a DAG run using the original run's log file.
func executeRetry(ctx *Context, dag *core.DAG, status *exec.DAGRunStatus, rootRun exec.DAGRunRef, stepName, workerID string) error {
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

	dr, err := ctx.dagStore(dagStoreConfig{
		SearchPaths:           []string{filepath.Dir(dag.Location)},
		SkipDirectoryCreation: workerID != "local",
	})
	if err != nil {
		return fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	as := ctx.agentStores()

	agentInstance := agent.New(
		status.DAGRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		agent.Options{
			RetryTarget:      status,
			ParentDAGRun:     status.Parent,
			ProgressDisplay:  shouldEnableProgress(ctx),
			StepRetry:        stepName,
			WorkerID:         workerID,
			DAGRunStore:      ctx.DAGRunStore,
			ServiceRegistry:  ctx.ServiceRegistry,
			RootDAGRun:       rootRun,
			PeerConfig:       ctx.Config.Core.Peer,
			TriggerType:      core.TriggerTypeRetry,
			DefaultExecMode:  ctx.Config.DefaultExecMode,
			AgentConfigStore: as.ConfigStore,
			AgentModelStore:  as.ModelStore,
			AgentMemoryStore: as.MemoryStore,
			AgentSkillStore:  as.SkillStore,
			AgentSoulStore:   as.SoulStore,
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, dag, status.DAGRunID, logFile)
}
