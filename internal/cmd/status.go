package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdStatus() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "status --workflow-id=abc123 <workflow name>",
			Short: "Display the current status of a workflow",
			Long: `Show real-time status information for a specified workflow.

Flags:
	--workflow-id string (optional) Unique identifier for tracking the execution.

Example:
  dagu status my_dag
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	workflowIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	workflowID, err := ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var run models.Run
	if workflowID != "" {
		// Retrieve the previous run's record for the specified workflow ID.
		ref := digraph.NewWorkflowRef(name, workflowID)
		r, err := ctx.HistoryRepo.FindRun(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find run data for workflow ID %s: %w", workflowID, err)
		}
		run = r
	} else {
		r, err := ctx.HistoryRepo.LatestRun(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest run data for DAG %s: %w", name, err)
		}
		run = r
	}

	dag, err := run.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from run data: %w", err)
	}

	status, err := ctx.HistoryMgr.GetDAGRealtimeStatus(ctx, dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status.String())

	return nil
}
