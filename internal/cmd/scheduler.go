package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func SchedulerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>] [--config=<config file>]`,
		PreRunE: func(cmd *cobra.Command, _ []string) error {
			return bindCommonFlags(cmd, []string{"dags"})
		},
		RunE: wrapRunE(runScheduler),
	}

	initCommonFlags(cmd, []commandLineFlag{dagsFlag})

	return cmd
}

func runScheduler(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup(cmd.Context(), false)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
		setup.cfg.Paths.DAGsDir = dagsDir
	}

	ctx := setup.ctx
	logger.Info(ctx, "Scheduler initialization", "specsDirectory", setup.cfg.Paths.DAGsDir, "logFormat", setup.cfg.Global.LogFormat)

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
