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
	workflow, err := digraph.ParseWorkflowRef(workflowRef)
	if err != nil {
		return fmt.Errorf("failed to parse workflow reference %s: %w", workflowRef, err)
	}
	return dequeueWorkflow(ctx, workflow)
}

// dequeueWorkflow dequeues a workflow to the queue.
func dequeueWorkflow(ctx *Context, workflow digraph.WorkflowRef) error {
	run, err := ctx.HistoryStore.FindRun(ctx, workflow)
	if err != nil {
		return fmt.Errorf("failed to find the record for workflow ID %s: %w", workflow.WorkflowID, err)
	}

	status, err := run.ReadStatus(ctx)
	if err != nil {
		return fmt.Errorf("failed to read status: %w", err)
	}

	if status.Status != scheduler.StatusQueued {
		// If the status is not queued, return an error
		return fmt.Errorf("workflow %s is not in queued status but %s", workflow.WorkflowID, status.Status)
	}

	dag, err := run.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read dag: %w", err)
	}

	// Make sure the workflow is not running at least locally
	latestStatus, err := ctx.HistoryMgr.GetDAGRealtimeStatus(ctx, dag, workflow.WorkflowID)
	if err != nil {
		return fmt.Errorf("failed to get latest status: %w", err)
	}
	if latestStatus.Status != scheduler.StatusQueued {
		return fmt.Errorf("workflow %s is not in queued status but %s", workflow.WorkflowID, latestStatus.Status)
	}

	// Make the workflow status to cancelled
	status.Status = scheduler.StatusCancel

	if err := run.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer func() {
		_ = run.Close(ctx.Context)
	}()
	if err := run.Write(ctx.Context, *status); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Dequeue the workflow
	if _, err = ctx.QueueStore.DequeueByWorkflowID(ctx.Context, workflow.Name, workflow.WorkflowID); err != nil {
		return fmt.Errorf("failed to dequeue workflow %s: %w", workflow.WorkflowID, err)
	}

	logger.Info(ctx.Context, "Dequeued workflow",
		"workflowName", workflow.Name,
		"workflowId", workflow.WorkflowID,
	)

	return nil
}
