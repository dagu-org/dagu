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
			Use:   "stop --workflow-id=abc123 name",
			Short: "Stop a running workflow",
			Long: `Gracefully terminate an active workflow.

Flags:
  --workflow-id string   (optional) Unique identifier for tracking the restart execution.

This command stops all running tasks of the specified workflow, ensuring resources are properly released.
If workflow ID is not provided, it will find the dag definition by name and stop the currently running workflow.

Example:
  dagu stop --workflow-id=abc123 name
`,
			Args: cobra.ExactArgs(1),
		}, stopFlags, runStop,
	)
}

var stopFlags = []commandLineFlag{
	workflowIDFlagStop,
}

func runStop(ctx *Context, args []string) error {
	reqID, err := ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var dag *digraph.DAG
	if reqID != "" {
		// Retrieve the previous run's history record for the specified workflow ID.
		ref := digraph.NewExecRef(name, reqID)
		rec, err := ctx.HistoryRepo.Find(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the record for workflow ID %s: %w", reqID, err)
		}

		d, err := rec.ReadDAG(ctx)
		if err != nil {
			return fmt.Errorf("failed to read DAG from history record: %w", err)
		}
		dag = d
	} else {
		d, err := digraph.Load(ctx, args[0], digraph.WithBaseConfig(ctx.Config.Paths.BaseConfig))
		if err != nil {
			return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
		}
		dag = d
	}

	logger.Info(ctx, "workflow is stopping", "dag", dag.Name)

	if err := ctx.HistoryMgr.Stop(ctx, dag, reqID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "workflow stopped", "dag", dag.Name)
	return nil
}
