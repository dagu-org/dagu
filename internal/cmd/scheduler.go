package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdScheduler() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler [flags]",
		Short: "Start the scheduler process",
		Long: `Launch the scheduler process that monitors and triggers DAG executions based on cron schedules.

Example:
  dagu scheduler --dags=/path/to/dags

This process runs continuously to automatically execute scheduled DAGs.
`,
		RunE: wrapRunE(runScheduler),
	}
	initFlags(cmd, schedulerFlags...)
	return cmd
}

var schedulerFlags = []commandLineFlag{dagsFlag}

func runScheduler(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup(cmd, schedulerFlags, false)
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
