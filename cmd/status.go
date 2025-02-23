package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func statusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status /path/to/spec.yaml",
		Short: "Display current status of the DAG",
		Long:  `dagu status /path/to/spec.yaml`,
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return bindCommonFlags(cmd, nil)
		},
		RunE: wrapRunE(runStatus),
	}

	initCommonFlags(cmd, nil)

	return cmd
}

func runStatus(cmd *cobra.Command, args []string) error {
	setup, err := createSetup(cmd.Context(), false)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.ctx

	dag, err := digraph.Load(ctx, args[0], digraph.WithBaseConfig(setup.cfg.Paths.BaseConfig))
	if err != nil {
		logger.Error(ctx, "Failed to load DAG", "path", args[0], "err", err)
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	cli, err := setup.client()
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
