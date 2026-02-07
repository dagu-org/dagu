package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
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
	namespaceFlag,
}

func runStop(ctx *Context, args []string) error {
	namespaceName, dagName, err := ctx.ResolveNamespaceFromArg(args[0])
	if err != nil {
		return err
	}

	dagRunID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to get dag-run ID: %w", err)
	}

	name, err := extractDAGName(ctx, dagName)
	if err != nil {
		return fmt.Errorf("failed to extract DAG name: %w", err)
	}

	var dag *core.DAG
	if dagRunID != "" {
		ref := exec.NewDAGRunRef(name, dagRunID)
		rec, err := ctx.DAGRunStore.FindAttempt(ctx, ref)
		if err != nil {
			return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRunID, err)
		}
		dag, err = rec.ReadDAG(ctx)
		if err != nil {
			return fmt.Errorf("failed to read DAG from history record: %w", err)
		}
	} else {
		var err error
		dag, err = spec.Load(ctx, dagName, spec.WithBaseConfig(ctx.Config.Paths.BaseConfig), spec.WithDAGsDir(ctx.NamespacedDAGsDir()))
		if err != nil {
			return fmt.Errorf("failed to load DAG from %s: %w", dagName, err)
		}
	}

	dag.Namespace = namespaceName

	logger.Info(ctx, "Dag-run is stopping", tag.DAG(dag.Name))

	if err := ctx.DAGRunMgr.Stop(ctx, dag, dagRunID); err != nil {
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "Dag-run stopped", tag.DAG(dag.Name))
	return nil
}
