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
			Use:   "status [flags] <DAG name>",
			Short: "Display the current status of a DAG-run",
			Long: `Show real-time status information for a specified DAG-run instance.

This command retrieves and displays the current execution status of a DAG-run,
including its state (running, completed, failed), process ID, and other relevant details.

Flags:
  --run-id string (optional) Unique identifier of the DAG-run to check.
                                 If not provided, it will show the status of the
                                 most recent DAG-run for the given name.

Example:
  dagu status --run-id=abc123 my_dag
  dagu status my_dag  # Shows status of the most recent DAG-run
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	dagRunIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get DAG-run ID: %w", err)
	}

	name := args[0]

	var attempt models.DAGRunAttempt
	if dagRunID != "" {
		// Retrieve the previous run's record for the specified DAG-run ID.
		dagRunRef := digraph.NewDAGRunRef(name, dagRunID)
		att, err := ctx.DAGRunStore.FindAttempt(ctx, dagRunRef)
		if err != nil {
			return fmt.Errorf("failed to find run data for DAG-run ID %s: %w", dagRunID, err)
		}
		attempt = att
	} else {
		r, err := ctx.DAGRunStore.LatestAttempt(ctx, name)
		if err != nil {
			return fmt.Errorf("failed to find the latest run data for DAG %s: %w", name, err)
		}
		attempt = r
	}

	dag, err := attempt.ReadDAG(ctx)
	if err != nil {
		return fmt.Errorf("failed to read DAG from run data: %w", err)
	}

	status, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status.String())

	return nil
}
