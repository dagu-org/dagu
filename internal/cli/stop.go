package cli

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/builder"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func Stop() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "stop [flags] <DAG name>",
			Short: "Stop a running DAG-run gracefully",
			Long: `Gracefully terminate an active DAG-run instance.

This command sends termination signals to all running tasks of the specified DAG-run,
ensuring resources are properly released and cleanup handlers are executed. It waits
for tasks to complete their shutdown procedures before exiting.

Flags:
  --run-id string   (optional) Unique identifier of the DAG-run to stop.
                                   If not provided, it will find and stop the currently
                                   running DAG-run by the given DAG definition name.

Example:
  dagu stop --run-id=abc123 my_dag
`,
			Args: cobra.ExactArgs(1),
		}, stopFlags, runStop,
	)
}

var stopFlags = []commandLineFlag{
	dagRunIDFlagStop,
}

func runStop(ctx *Context, args []string) error {
	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	name, err := extractDAGName(ctx, args[0])
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	var dag *core.DAG
	if dagRunID != "" {
		// Retrieve the previous run's history record for the specified dag-run ID.
		ref := core.NewDAGRunRef(name, dagRunID)
		rec, err := ctx.DAGRunStore.FindAttempt(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
		}

		d, err := rec.ReadDAG(ctx)
		if err != nil {
			return fmt.Errorf("failed to read DAG from history record: %w", err)
		}
		dag = d
	} else {
		d, err := builder.Load(ctx, args[0], builder.WithBaseConfig(ctx.Config.Paths.BaseConfig))
		if err != nil {
			return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
		}
		dag = d
	}

	logger.Info(ctx, "dag-run is stopping", "dag", dag.Name)

	if err := ctx.DAGRunMgr.Stop(ctx, dag, dagRunID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "dag-run stopped", "dag", dag.Name)
	return nil
}
