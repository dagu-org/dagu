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
			Use:   "stop [flags] <DAG definition or workflow name>",
			Short: "Stop a running workflow",
			Long: `Gracefully terminate an active workflow instance.

This command sends termination signals to all running tasks of the specified workflow,
ensuring resources are properly released and cleanup handlers are executed. It waits
for tasks to complete their shutdown procedures before exiting.

Flags:
  --workflow-id string   (optional) Unique identifier of the workflow to stop.
                                   If not provided, it will find and stop the currently
                                   running workflow by the given DAG definition name.

Example:
  dagu stop --workflow-id=abc123 my_dag
`,
			Args: cobra.ExactArgs(1),
		}, stopFlags, runStop,
	)
}

var stopFlags = []commandLineFlag{
	workflowIDFlagStop,
}

func runStop(ctx *Context, args []string) error {
	workflowID, err := ctx.Command.Flags().GetString("workflow-id")
	if err != nil {
		return fmt.Errorf("failed to get workflow ID: %w", err)
	}

	name := args[0]

	var dag *digraph.DAG
	if workflowID != "" {
		// Retrieve the previous run's history record for the specified workflow ID.
		ref := digraph.NewWorkflowRef(name, workflowID)
		rec, err := ctx.HistoryStore.FindRun(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the record for workflow ID %s: %w", workflowID, err)
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

	logger.Info(ctx, "Workflow is stopping", "dag", dag.Name)

	if err := ctx.HistoryMgr.Stop(ctx, dag, workflowID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "Workflow stopped", "dag", dag.Name)
	return nil
}
