package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdDequeue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dequeue [flags]",
			Short: "Dequeue a workflow to the queue.",
			Long: `Dequeue a workflow to the queue.

Example:
	dagu dequeue --workflow=my_workflow_name:my_workflow_id
`,
		}, dequeueFlags, runDequeue,
	)
}

var dequeueFlags = []commandLineFlag{paramsFlag, workflowFlagDequeue}

func runDequeue(ctx *Context, _ []string) error {
	// Get workflow ID from flags
	workflowRef, _ := ctx.StringParam("workflow")
	workflow, err := digraph.ParseDAGRunRef(workflowRef)
	if err != nil {
		return fmt.Errorf("failed to parse workflow reference %s: %w", workflowRef, err)
	}
	return dequeueWorkflow(ctx, workflow)
}

// dequeueWorkflow dequeues a workflow to the queue.
func dequeueWorkflow(ctx *Context, workflow digraph.DAGRunRef) error {
	attempt, err := ctx.HistoryStore.FindAttempt(ctx, workflow)
	if err != nil {
		return fmt.Errorf("failed to find the record for workflow ID %s: %w", workflow.ID, err)
	}

	status, err := attempt.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	if status.Status != scheduler.StatusQueued {
		// If the status is not queued, return an error
		return fmt.Errorf("workflow %s is not in queued status but %s", workflow.ID, status.Status)
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read dag: %w", err)
	}

	// Make sure the workflow is not running at least locally
	latestStatus, err := ctx.HistoryMgr.GetCurrentStatus(ctx, dag, workflow.ID)
	if err != nil {
		return fmt.Errorf("failed to get latest status: %w", err)
	}
	if latestStatus.Status != scheduler.StatusQueued {
		return fmt.Errorf("workflow %s is not in queued status but %s", workflow.ID, latestStatus.Status)
	}

	// Make the workflow status to cancelled
	status.Status = scheduler.StatusCancel

	if err := attempt.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = attempt.Close(ctx.Context)
	}()
	if err := attempt.Write(ctx.Context, *status); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Dequeue the workflow
	if _, err = ctx.QueueStore.DequeueByDAGRunID(ctx.Context, workflow.Name, workflow.ID); err != nil {
		return fmt.Errorf("failed to dequeue workflow %s: %w", workflow.ID, err)
	}

	logger.Info(ctx.Context, "Dequeued workflow",
		"workflowName", workflow.Name,
		"dagRunId", workflow.ID,
	)

	return nil
}
