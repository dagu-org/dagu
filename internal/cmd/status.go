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
			Use:   "status [flags] <workflow name>",
			Short: "Display the current status of a workflow",
			Long: `Show real-time status information for a specified workflow instance.

This command retrieves and displays the current execution status of a workflow,
including its state (running, completed, failed), process ID, and other relevant details.
It connects to the workflow's agent to get the most up-to-date information.

Flags:
  --workflow-id string (optional) Unique identifier of the workflow to check.
                                 If not provided, it will show the status of the
                                 most recent workflow for the given name.

Example:
  dagu status --workflow-id=abc123 my_dag
  dagu status my_dag  # Shows status of the most recent workflow
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	workflowIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	workflowID, err := ctx.StringParam("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var attempt models.DAGRunAttempt
	if workflowID != "" {
		// Retrieve the previous run's record for the specified workflow ID.
		dagRunRef := digraph.NewDAGRunRef(name, workflowID)
		att, err := ctx.dagRunStore.FindAttempt(ctx, dagRunRef)
		if err != nil {
			return fmt.Errorf("failed to find run data for workflow ID %s: %w", workflowID, err)
		}
		attempt = att
	} else {
		r, err := ctx.dagRunStore.LatestAttempt(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest run data for DAG %s: %w", name, err)
		}
		attempt = r
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from run data: %w", err)
	}

	status, err := ctx.dagRunMgr.GetCurrentStatus(ctx, dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status.String())

	return nil
}
