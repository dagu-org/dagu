package cmd

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/spf13/cobra"
)

func Enqueue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "enqueue [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Enqueue a DAG-run to the queue.",
			Long: `Enqueue a DAG-run to the queue.

Examples:
	dagu enqueue --run-id=run_id my_dag -- P1=foo P2=bar
	dagu enqueue --name my_custom_name my_dag.yaml -- P1=foo P2=bar
`,
		}, enqueueFlags, runEnqueue,
	)
}

var enqueueFlags = []commandLineFlag{paramsFlag, nameFlag, dagRunIDFlag, queueFlag}

func runEnqueue(ctx *Context, args []string) error {
	// Get Run ID from the context or generate a new one
	runID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get Run ID: %w", err)
	}

	if runID == "" {
		// Generate a new Run ID if not provided
		runID, err = genRunID()
		if err != nil {
			return fmt.Errorf("failed to generate Run ID: %w", err)
		}
	} else if err := validateRunID(runID); err != nil {
		return fmt.Errorf("invalid Run ID: %w", err)
	}

	// Get queue override from the context
	queueOverride, err := ctx.StringParam("queue")
	if err != nil {
		return fmt.Errorf("failed to get queue override: %w", err)
	}

	// Load parameters and DAG (enqueue is always for root DAGs, not sub-DAGs)
	dag, _, err := loadDAGWithParams(ctx, args, false)
	if err != nil {
		return err
	}

	// Apply queue override if provided
	if queueOverride != "" {
		dag.Queue = queueOverride
	}

	// Check queued DAG-runs
	queuedRuns, err := ctx.QueueStore.ListByDAGName(ctx, dag.ProcGroup(), dag.Name)
	if err != nil {
		return fmt.Errorf("failed to read queue: %w", err)
	}

	// If the DAG has a queue configured and maxActiveRuns > 1, ensure the number
	// of active runs in the queue does not exceed this limit.
	// No need to check if maxActiveRuns <= 1 for enqueueing as queue level
	// maxConcurrency will be the only cap.
	if dag.Queue != "" && dag.MaxActiveRuns > 1 && len(queuedRuns) >= dag.MaxActiveRuns {
		// The same DAG is already in the queue
		return fmt.Errorf("DAG %s is already in the queue (maxActiveRuns=%d), cannot enqueue", dag.Name, dag.MaxActiveRuns)
	}

	return enqueueDAGRun(ctx, dag, runID)
}

// enqueueDAGRun enqueues a dag-run to the queue.
func enqueueDAGRun(ctx *Context, dag *core.DAG, dagRunID string) error {
	// Queued dag-runs must not have a location because it is used to generate
	// unix pipe. If two DAGs has same location, they can not run at the same time.
	// Queued DAGs can be run at the same time depending on the `maxActiveRuns` setting.
	dag.Location = ""

	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}
	logFile, err := ctx.GenLogFileName(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to generate log file name: %w", err)
	}

	dagRun := execution.NewDAGRunRef(dag.Name, dagRunID)

	// Check if the dag-run is already existing in the history store
	if _, err = ctx.DAGRunStore.FindAttempt(ctx, dagRun); err == nil {
		return fmt.Errorf("DAG %q with ID %q already exists", dag.Name, dagRunID)
	}

	att, err := ctx.DAGRunStore.CreateAttempt(ctx.Context, dag, time.Now(), dagRunID, execution.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

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

	// As a prototype, we save the status to the database to enqueue the dag-run.
	// This could be changed to save to a queue file in the future
	dagStatus := transform.NewStatusBuilder(dag).Create(dagRunID, core.Queued, 0, time.Time{}, opts...)

	if err := att.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = att.Close(ctx.Context)
	}()
	if err := att.Write(ctx.Context, dagStatus); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Enqueue the dag-run to the queue
	// Use ProcGroup() to get the correct queue name (respects dag.Queue if set, otherwise dag.Name)
	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.ProcGroup(), execution.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue dag-run: %w", err)
	}

	logger.Info(ctx.Context, "Enqueued dag-run",
		tag.DAG(dag.Name),
		tag.RunID(dagRunID),
		slog.Any("params", dag.Params),
	)

	return nil
}
