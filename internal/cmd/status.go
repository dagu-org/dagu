package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/runstore"
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
	requestIDFlagStatus,
}

func runStatus(ctx *Context, args []string) error {
	requestID, err := ctx.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	dagName := args[0]

	var record runstore.Record
	if requestID != "" {
		// Retrieve the previous run's runstore record for the specified request ID.
		r, err := ctx.runStore().FindByRequestID(ctx, dagName, requestID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "requestID", requestID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", requestID, err)
		}
		record = r
	} else {
		r, err := ctx.runStore().Latest(ctx, dagName)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve latest runstore record", "dagName", dagName, "err", err)
			return fmt.Errorf("failed to retrieve latest runstore record for DAG %s: %w", dagName, err)
		}
		record = r
	}

	dag, err := record.ReadDAG(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG from record", "dagName", dagName, "err", err)
	}

	cli, err := ctx.Client()
	if err != nil {
		logger.Error(ctx, "failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	status, err := cli.GetRealtimeStatus(ctx, dag, requestID)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve current status", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status.String())

	return nil
}
