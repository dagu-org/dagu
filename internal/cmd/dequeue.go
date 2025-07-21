package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdDequeue() *cobra.Command {
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
	dagRun, err := digraph.ParseDAGRunRef(dagRunRef)
	if err != nil {
		return fmt.Errorf("failed to parse dag-run reference %s: %w", dagRunRef, err)
	}
	return dequeueDAGRun(ctx, dagRun)
}

// dequeueDAGRun dequeues a dag-run from the queue.
func dequeueDAGRun(ctx *Context, dagRun digraph.DAGRunRef) error {
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

	// Make the status as canceled
	dagStatus.Status = status.Cancel

	if err := attempt.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = attempt.Close(ctx.Context)
	}()
	if err := attempt.Write(ctx.Context, *dagStatus); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Dequeue the dag-run from the queue
	if _, err = ctx.QueueStore.DequeueByDAGRunID(ctx.Context, dagRun.Name, dagRun.ID); err != nil {
		return fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
	}

	logger.Info(ctx.Context, "Dequeued dag-run",
		"dag", dagRun.Name,
		"runId", dagRun.ID,
	)

	return nil
}
