package main

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func schedulerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		RunE:  wrapRunE(runScheduler),
	}

	cmd.Flags().StringP(
		"dags",
		"d",
		"",
		"location of DAG files (default is $HOME/.config/dagu/dags)",
	)
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}

func runScheduler(cmd *cobra.Command, _ []string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	setup := newSetup(cfg)

	ctx := setup.loggerContext(cmd.Context(), false)

	// Update DAGs directory if specified
	if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
		cfg.Paths.DAGsDir = dagsDir
	}

	logger.Info(ctx, "Scheduler initialization", "specsDirectory", cfg.Paths.DAGsDir, "logFormat", cfg.LogFormat)

	scheduler, err := setup.scheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler in directory %s: %w",
			cfg.Paths.DAGsDir, err)
	}

	return nil
}
