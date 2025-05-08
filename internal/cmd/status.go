package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/spf13/cobra"
)

func CmdStatus() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "status --request-id=abc123 dagName",
			Short: "Display the current status of a DAG",
			Long: `Show real-time status information for a specified DAG run.

Flags:
	--request-id string (optional) Unique identifier for tracking the execution.

Example:
  dagu status my_dag.yaml
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{
	reqIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	reqID, err := ctx.Command.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	name := args[0]

	var record models.Record
	if reqID != "" {
		// Retrieve the previous run's record for the specified request ID.
		r, err := ctx.HistoryRepo.Find(ctx, name, reqID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "reqId", reqID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", reqID, err)
		}
		record = r
	} else {
		r, err := ctx.HistoryRepo.Latest(ctx, name)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve latest run record", "dagName", name, "err", err)
			return fmt.Errorf("failed to retrieve latest run record for DAG %s: %w", name, err)
		}
		record = r
	}

	dag, err := record.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG from record", "dagName", name, "err", err)
	}

	status, err := ctx.HistoryMgr.GetRealtimeStatus(ctx, dag, reqID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve current status", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status.String())

	return nil
}
