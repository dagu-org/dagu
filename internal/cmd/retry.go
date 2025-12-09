package cmd

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/dagu-org/dagu/internal/runtime/transform"
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

var retryFlags = []commandLineFlag{dagRunIDFlagRetry, stepNameForRetry, disableMaxActiveRuns, noQueueFlag}

func runRetry(ctx *Context, args []string) error {
	dagRunID, _ := ctx.StringParam("run-id")
	stepName, _ := ctx.StringParam("step")
	disableMaxActiveRuns := ctx.Command.Flags().Changed("disable-max-active-runs")

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

	// Check if queue is disabled via config or flag
	queueDisabled := !ctx.Config.Queues.Enabled || ctx.Command.Flags().Changed("no-queue")

	// Check if this DAG should be distributed to workers
	// If the DAG has a workerSelector and the queue is not disabled,
	// enqueue it so the scheduler can dispatch it to a worker.
	// The --no-queue flag acts as a circuit breaker to prevent infinite loops
	// when the worker executes the dispatched retry task.
	if !queueDisabled && len(dag.WorkerSelector) > 0 {
		logger.Info(ctx, "DAG has workerSelector, enqueueing retry for distributed execution", slog.Any("worker-selector", dag.WorkerSelector))

		// Enqueue the retry - must create new attempt with status "Queued"
		// so the scheduler will process it
		if err := enqueueRetry(ctx, dag, dagRunID); err != nil {
			return fmt.Errorf("failed to enqueue retry: %w", err)
		}

		logger.Info(ctx.Context, "Retry enqueued")
		return nil
	}

	// Try lock proc store to avoid race
	if err := ctx.ProcStore.Lock(ctx, dag.ProcGroup()); err != nil {
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
	proc, err := ctx.ProcStore.Acquire(ctx, dag.ProcGroup(), execution.NewDAGRunRef(dag.Name, dagRunID))
	if err != nil {
		logger.Debug(ctx, "Failed to acquire process handle", tag.Error(err))
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

// executeRetry prepares and runs a retry of a DAG run by opening the original run's log,
// loading the DAG environment, initializing the DAG store, creating an agent configured
// for retry (including step-level retry if provided), and invoking the shared agent executor.
// It returns an error if any setup step fails or if the agent execution fails.
func executeRetry(ctx *Context, dag *core.DAG, status *execution.DAGRunStatus, rootRun execution.DAGRunRef, stepName string) error {
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
		},
	)

	// Use the shared agent execution function
	return ExecuteAgent(ctx, agentInstance, dag, status.DAGRunID, logFile)
}

// enqueueRetry creates a new attempt for retry and enqueues it for execution
func enqueueRetry(ctx *Context, dag *core.DAG, dagRunID string) error {
	// Queued dag-runs must not have a location because it is used to generate
	// unix pipe. If two DAGs has same location, they can not run at the same time.
	// Queued DAGs can be run at the same time depending on the `maxActiveRuns` setting.
	dag.Location = ""

	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}

	// Create a new attempt for retry
	att, err := ctx.DAGRunStore.CreateAttempt(ctx.Context, dag, time.Now(), dagRunID, execution.NewDAGRunAttemptOptions{
		Retry: true,
	})
	if err != nil {
		return fmt.Errorf("failed to create retry attempt: %w", err)
	}

	// Generate log file name
	logFile, err := ctx.GenLogFileName(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to generate log file name: %w", err)
	}

	// Create status for the new attempt with "Queued" status
	opts := []transform.StatusOption{
		transform.WithLogFilePath(logFile),
		transform.WithAttemptID(att.ID()),
		transform.WithPreconditions(dag.Preconditions),
		transform.WithQueuedAt(stringutil.FormatTime(time.Now())),
		transform.WithHierarchyRefs(
			execution.NewDAGRunRef(dag.Name, dagRunID),
			execution.DAGRunRef{},
		),
	}

	dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{}, opts...)

	// Write the status
	if err := att.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open attempt: %w", err)
	}
	defer func() {
		_ = att.Close(ctx.Context)
	}()
	if err := att.Write(ctx.Context, dagStatus); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Enqueue the DAG run
	dagRun := execution.NewDAGRunRef(dag.Name, dagRunID)
	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.ProcGroup(), execution.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue: %w", err)
	}

	logger.Info(ctx, "Retry attempt created and enqueued", tag.AttemptID(att.ID()))

	return nil
}