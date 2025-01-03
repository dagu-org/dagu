package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func stopCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stop /path/to/spec.yaml",
		Short: "Stop the running DAG",
		Long:  `dagu stop /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		RunE:  wrapRunE(runStop),
	}
	return cmd
}

func runStop(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	setup := newSetup(cfg)

	ctx := setup.loggerContext(cmd.Context(), false)

	dag, err := digraph.Load(cmd.Context(), args[0], digraph.WithBaseConfig(cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	logger.Info(ctx, "DAG is stopping", "dag", dag.Name)

	cli, err := setup.client()
	if err != nil {
		logger.Error(ctx, "failed to initialize client", "err", err)
		return fmt.Errorf("failed to initialize client: %w", err)
	}

	if err := cli.Stop(cmd.Context(), dag); err != nil {
		logger.Error(ctx, "Failed to stop DAG", "dag", dag.Name, "err", err)
		return fmt.Errorf("failed to stop DAG: %w", err)
	}

	logger.Info(ctx, "DAG stopped", "dag", dag.Name)
	return nil
}
