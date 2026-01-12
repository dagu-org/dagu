package cmd

import (
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/spf13/cobra"
)

func Dequeue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dequeue [flags] <queue-name>",
			Short: "Dequeue a DAG-run from the specified queue",
			Long: `Dequeue a DAG-run from the queue.

Example:
	dagu dequeue default --dag-run=dag_name:my_dag_run_id
	dagu dequeue default
`,
			Args: cobra.ExactArgs(1),
		}, dequeueFlags, runDequeue,
	)
}

var dequeueFlags = []commandLineFlag{paramsFlag, dagRunFlagDequeue}

func runDequeue(ctx *Context, args []string) error {
	queueName := args[0]

	// Get dag-run reference from the context
	dagRunRef, _ := ctx.StringParam("dag-run")
	if dagRunRef == "" {
		return dequeueFirst(ctx, queueName)
	}

	dagRun, err := exec.ParseDAGRunRef(dagRunRef)
	if err != nil {
		return fmt.Errorf("failed to parse dag-run reference %s: %w", dagRunRef, err)
	}
	return dequeueDAGRun(ctx, queueName, dagRun, false)
}

// dequeueFirst dequeues the first DAG run from the named queue and processes that run as aborted.
//
// It returns an error if queues are disabled, if removing an item from the queue fails,
// if the queue is empty, if retrieving the dequeued item's DAG-run data fails, or if
// processing the dequeued DAG run fails.
func dequeueFirst(ctx *Context, queueName string) error {
	// Check if queues are enabled
	if !ctx.Config.Queues.Enabled {
		return fmt.Errorf("queues are disabled in configuration")
	}
	item, err := ctx.QueueStore.DequeueByName(ctx.Context, queueName)
	if err != nil {
		return fmt.Errorf("failed to dequeue from queue %s: %w", queueName, err)
	}
	if item == nil {
		return fmt.Errorf("no dag-run found in queue %s", queueName)
	}

	data, err := item.Data()
	if err != nil {
		return fmt.Errorf("failed to get dag-run data: %w", err)
	}
	return dequeueDAGRun(ctx, queueName, *data, true)
}

// dequeueDAGRun dequeues a dag-run from the queue.
func dequeueDAGRun(ctx *Context, queueName string, dagRun exec.DAGRunRef, alreadyDequeued bool) error {
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

	if dagStatus.Status != core.Queued {
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
	if latestStatus.Status != core.Queued {
		return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, latestStatus.Status)
	}

	// Dequeue the dag-run from the queue if we have not done so already
	if !alreadyDequeued {
		if _, err = ctx.QueueStore.DequeueByDAGRunID(ctx.Context, queueName, dagRun); err != nil {
			return fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
		}
	}

	// Mark the execution as aborted now that it is dequeued
	dagStatus.Status = core.Aborted

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

	// Hide the aborted attempt to preserve the previous state
	if err := attempt.Hide(ctx.Context); err != nil {
		return fmt.Errorf("failed to hide aborted attempt: %w", err)
	}

	// Read the latest attempt and if it's NotStarted, we can remove the DAGRun from the store
	// as it only has the queued status and no other attempts.
	_, err = ctx.DAGRunStore.FindAttempt(ctx, dagRun)
	if errors.Is(err, exec.ErrNoStatusData) {
		if err := ctx.DAGRunStore.RemoveDAGRun(ctx, dagRun); err != nil {
			return fmt.Errorf("failed to remove dag-run %s from store: %w", dagRun.ID, err)
		}
	}

	logger.Info(ctx.Context, "Dequeued dag-run",
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
		tag.Queue(queueName),
	)

	return nil
}
