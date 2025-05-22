package cmd

import (
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdEnqueue() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "enqueue [flags]",
			Short: "Enqueue a workflow to the queue.",
			Long: `Enqueue a workflow to the queue.

Example:
	dagu enqueue --workflow-id my_workflow_id my_dag -- P1=foo P2=bar
`,
		}, enqueueFlags, runEnqueue,
	)
}

var enqueueFlags = []commandLineFlag{paramsFlag, workflowIDFlagStart}

func runEnqueue(ctx *Context, args []string) error {
	// Get workflow ID from flags
	workflowID, err := ctx.StringParam("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	if workflowID == "" {
		// Generate a new workflow ID
		workflowID, err = genWorkflowID()
		if err != nil {
			return fmt.Errorf("failed to generate workflow ID: %w", err)
		}
	} else if err := validateWorkflowID(workflowID); err != nil {
		return fmt.Errorf("invalid workflow ID: %w", err)
	}

	// Load parameters and DAG
	dag, _, err := loadDAGWithParams(ctx, args)
	if err != nil {
		return err
	}

	return enqueueWorkflow(ctx, dag, workflowID)
}

// enqueueWorkflow enqueues a workflow to the queue.
func enqueueWorkflow(ctx *Context, dag *digraph.DAG, workflowID string) error {
	logFile, err := ctx.GenLogFileName(dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to generate log file name: %w", err)
	}

	run, err := ctx.HistoryStore.CreateRun(ctx.Context, dag, time.Now(), workflowID, models.NewRunOptions{})
	if err != nil {
		return fmt.Errorf("failed to create run: %w", err)
	}

	opts := []models.StatusOption{
		models.WithLogFilePath(logFile),
		models.WithRunID(run.ID()),
		models.WithPreconditions(dag.Preconditions),
	}

	// As a prototype, we save the status to the database to enqueue the workflow
	// This could be changed to save to a queue file in the future
	status := models.NewStatusBuilder(dag).Create(workflowID, scheduler.StatusQueued, 0, time.Now(), opts...)

	if err := run.Open(ctx.Context); err != nil {
		return fmt.Errorf("failed to open run: %w", err)
	}
	defer run.Close(ctx.Context)
	if err := run.Write(ctx.Context, status); err != nil {
		return fmt.Errorf("failed to save status: %w", err)
	}

	// Enqueue the workflow
	if err := ctx.QueueStore.Enqueue(ctx.Context, dag.Name, models.QueuePriorityLow, digraph.NewWorkflowRef(dag.Name, workflowID)); err != nil {
		return fmt.Errorf("failed to enqueue workflow: %w", err)
	}

	logger.Info(ctx.Context, "Enqueued workflow",
		"workflowId", workflowID,
		"dagName", dag.Name,
		"params", dag.Params,
	)

	return nil
}
