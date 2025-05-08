package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStop() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "stop --request-id=abc123 dagName",
			Short: "Stop a running DAG",
			Long: `Gracefully terminate an active DAG run.

Flags:
  --request-id string   (optional) Unique identifier for tracking the restart execution.

This command stops all running tasks of the specified DAG, ensuring resources are properly released.
If request ID is not provided, it will find the current running DAG by name.

Example:
  dagu stop --request-id=abc123 dagName
`,
			Args: cobra.ExactArgs(1),
		}, stopFlags, runStop,
	)
}

var stopFlags = []commandLineFlag{
	requestIDFlagStop,
}

func runStop(ctx *Context, args []string) error {
	reqID, err := ctx.Command.Flags().GetString("request-id")
	if err != nil {
		return fmt.Errorf("failed to get request ID: %w", err)
	}

	name := args[0]

	var dag *digraph.DAG
	if reqID != "" {
		// Retrieve the previous run's history record for the specified request ID.
		rec, err := ctx.HistoryRepo.Find(ctx, name, reqID)
		if err != nil {
			logger.Error(ctx, "Failed to retrieve historical run", "requestID", reqID, "err", err)
			return fmt.Errorf("failed to retrieve historical run for request ID %s: %w", reqID, err)
		}

		d, err := rec.ReadDAG(ctx)
		if err != nil {
			logger.Error(ctx, "Failed to read DAG from history record", "err", err)
			return fmt.Errorf("failed to read DAG from history record: %w", err)
		}
		dag = d
	} else {
		d, err := digraph.Load(ctx, args[0], digraph.WithBaseConfig(ctx.Config.Paths.BaseConfig))
		if err != nil {
			logger.Error(ctx, "Failed to load DAG", "err", err)
			return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
		}
		dag = d
	}

	logger.Info(ctx, "DAG is stopping", "dag", dag.Name)

	if err := ctx.HistoryMgr.Stop(ctx, dag, reqID); err != nil {
		logger.Error(ctx, "Failed to stop DAG", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "DAG stopped", "dag", dag.Name)
	return nil
}
