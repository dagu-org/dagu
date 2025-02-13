package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func schedulerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "scheduler",
		Short:   "Start the scheduler",
		Long:    `dagu scheduler [--dags=<DAGs dir>] [--config=<config file>]`,
		PreRunE: bindSchedulerFlags,
		RunE:    wrapRunE(runScheduler),
	}

	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.config/dagu/dags)")
	cmd.Flags().StringP("config", "c", "", "config file (default is $HOME/.config/dagu/config.yaml)")
	return cmd
}

func bindSchedulerFlags(cmd *cobra.Command, _ []string) error {
	flags := []string{"dags", "config"}
	for _, flag := range flags {
		if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
			return fmt.Errorf("failed to bind flag %s: %w", flag, err)
		}
	}
	return nil
}

func runScheduler(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup()
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.loggerContext(cmd.Context(), false)

	// Update DAGs directory if specified
	if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
		setup.cfg.Paths.DAGsDir = dagsDir
	}

	logger.Info(ctx, "Scheduler initialization", "specsDirectory", setup.cfg.Paths.DAGsDir, "logFormat", setup.cfg.LogFormat)

	scheduler, err := setup.scheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler in directory %s: %w",
			setup.cfg.Paths.DAGsDir, err)
	}

	return nil
}
