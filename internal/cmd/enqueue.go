package cmd

import (
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/spf13/cobra"
)

func CmdEnqueue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "enqueue [flags]",
			Short: "Enqueue a DAG-run to the queue.",
			Long: `Enqueue a DAG-run to the queue.

Example:
	dagu enqueue --run-id=run_id my_dag -- P1=foo P2=bar
`,
		}, enqueueFlags, runEnqueue,
	)
}

var enqueueFlags = []commandLineFlag{paramsFlag, dagRunIDFlag}

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

	// Load parameters and DAG
	dag, _, err := loadDAGWithParams(ctx, args)
	if err != nil {
		return err
	}
	dag.Location = "" // Queued DAG-runs must not have a location

	return enqueueDAGRun(ctx, dag, runID)
}

// enqueueDAGRun enqueues a DAG-run to the queue.
func enqueueDAGRun(ctx *Context, dag *digraph.DAG, dagRunID string) error {
	logFile, err := ctx.GenLogFileName(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to generate log file name: %w", err)
	}

	dagRun := digraph.NewDAGRunRef(dag.Name, dagRunID)

	// Check if the DAG-run is already existing in the history store
	if _, err = ctx.DAGRunStore.FindAttempt(ctx, dagRun); err == nil {
		return fmt.Errorf("DAG %q with ID %q already exists", dag.Name, dagRunID)
	}

	att, err := ctx.DAGRunStore.CreateAttempt(ctx.Context, dag, time.Now(), dagRunID, models.NewDAGRunAttemptOptions{})
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	opts := []models.StatusOption{
		models.WithLogFilePath(logFile),
		models.WithAttemptID(att.ID()),
		models.WithPreconditions(dag.Preconditions),
		models.WithQueuedAt(stringutil.FormatTime(time.Now())),
		models.WithHierarchyRefs(
			digraph.NewDAGRunRef(dag.Name, dagRunID),
			digraph.DAGRunRef{},
		),
	}

	// As a prototype, we save the status to the database to enqueue the DAG-run.
	// This could be changed to save to a queue file in the future
	status := models.NewStatusBuilder(dag).Create(dagRunID, scheduler.StatusQueued, 0, time.Time{}, opts...)

	if err := att.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = att.Close(ctx.Context)
	}()
	if err := att.Write(ctx.Context, status); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Enqueue the DAG-run to the queue
	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.Name, models.QueuePriorityLow, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue DAG-run: %w", err)
	}

	logger.Info(ctx.Context, "Enqueued DAG-run",
		"dag", dag.Name,
		"dagRunId", dagRunID,
		"params", dag.Params,
	)

	return nil
}
