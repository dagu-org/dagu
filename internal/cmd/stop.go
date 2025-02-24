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
			Use:   "stop [flags] /path/to/spec.yaml",
			Short: "Stop a running DAG",
			Long: `Gracefully terminate an active DAG execution.

This command stops all running tasks of the specified DAG, ensuring resources are properly released.

Example:
  dagu stop my_dag.yaml
`,
			Args: cobra.ExactArgs(1),
		}, stopFlags, runStop,
	)
}

var stopFlags = []commandLineFlag{}

func runStop(cmd *Command, args []string) error {
	ctx := cmd.ctx

	dag, err := digraph.Load(ctx, args[0], digraph.WithBaseConfig(cmd.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	logger.Info(ctx, "DAG is stopping", "dag", dag.Name)

	cli, err := cmd.Client()
	if err != nil {
		logger.Error(ctx, "failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	if err := cli.Stop(cmd.ctx, dag); err != nil {
		logger.Error(ctx, "Failed to stop DAG", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "DAG stopped", "dag", dag.Name)
	return nil
}
