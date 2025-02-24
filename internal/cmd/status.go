package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStatus() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "status [flags] /path/to/spec.yaml",
			Short: "Display the current status of a DAG",
			Long: `Show real-time status information for a specified DAG execution.

Example:
  dagu status my_dag.yaml
`,
			Args: cobra.ExactArgs(1),
		}, statusFlags, runStatus,
	)
}

var statusFlags = []commandLineFlag{}

func runStatus(ctx *Context, args []string) error {
	dag, err := digraph.Load(ctx, args[0], digraph.WithBaseConfig(ctx.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", args[0], "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	cli, err := ctx.Client()
	if err != nil {
		logger.Error(ctx, "failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	status, err := cli.GetCurrentStatus(ctx, dag)
	if err != nil {
		logger.Error(ctx, "Failed to retrieve current status", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to retrieve current status: %w", err)
	}

	logger.Info(ctx, "Current status", "pid", status.PID, "status", status.Status)

	return nil
}
