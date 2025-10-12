package cli

import (
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func Dequeue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dequeue [flags]",
			Short: "Dequeue a DAG-run from the queue",
			Long: `Dequeue a DAG-run from the queue.

Example:
	dagu dequeue --dag-run=dag_name:my_dag_run_id
`,
		}, dequeueFlags, runDequeue,
	)
}

var dequeueFlags = []commandLineFlag{paramsFlag, dagRunFlagDequeue}

func runDequeue(ctx *Context, _ []string) error {
	// Get dag-run reference from the context
	dagRunRef, _ := ctx.StringParam("dag-run")
	dagRun, err := core.ParseDAGRunRef(dagRunRef)
	if err != nil {
		return fmt.Errorf("failed to parse dag-run reference %s: %w", dagRunRef, err)
	}
	return dequeueDAGRun(ctx, dagRun)
}

// dequeueDAGRun dequeues a dag-run from the queue.
func dequeueDAGRun(ctx *Context, dagRun core.DAGRunRef) error {
	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}
	attempt, err := ctx.DAGRunStore.FindAttempt(ctx, dagRun)
	if err != nil {
		return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRun.ID, err)
	}

	dagStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	if dagStatus.Status != status.Queued {
		// If the status is not queued, return an error
		return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, dagStatus.Status)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read dag: %w", err)
	}

	// Make sure the dag-run is not running at least locally
	latestStatus, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRun.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest status: %w", err)
	}
	if latestStatus.Status != status.Queued {
		return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, latestStatus.Status)
	}

	// Dequeue the dag-run from the queue
	if _, err = ctx.QueueStore.DequeueByDAGRunID(ctx.Context, dagRun.Name, dagRun.ID); err != nil {
		return fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
	}

	// Make the status as canceled
	dagStatus.Status = status.Cancel

	if err := attempt.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	if err := attempt.Write(ctx.Context, *dagStatus); err != nil {
		_ = attempt.Close(ctx.Context)
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Close the attempt before hiding
	if err := attempt.Close(ctx.Context); err != nil {
		return fmt.Errorf("failed to close attempt: %w", err)
	}

	// Hide the canceled attempt to preserve the previous state
	if err := attempt.Hide(ctx.Context); err != nil {
		return fmt.Errorf("failed to hide canceled attempt: %w", err)
	}

	// Read the latest attempt and if it's NotStarted, we can remove the DAGRun from the store
	// as it only has the queued status and no other attempts.
	_, err = ctx.DAGRunStore.FindAttempt(ctx, dagRun)
	if errors.Is(err, models.ErrNoStatusData) {
		if err := ctx.DAGRunStore.RemoveDAGRun(ctx, dagRun); err != nil {
			return fmt.Errorf("failed to remove dag-run %s from store: %w", dagRun.ID, err)
		}
	}

	logger.Info(ctx.Context, "Dequeued dag-run",
		"dag", dagRun.Name,
		"runId", dagRun.ID,
	)

	return nil
}
