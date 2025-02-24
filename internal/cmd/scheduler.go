package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdScheduler() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "scheduler [flags]",
			Short: "Start the scheduler process",
			Long: `Launch the scheduler process that monitors and triggers DAG executions based on cron schedules.

Example:
  dagu scheduler --dags=/path/to/dags

This process runs continuously to automatically execute scheduled DAGs.
`,
		}, schedulerFlags, runScheduler,
	)
}

var schedulerFlags = []commandLineFlag{dagsFlag}

func runScheduler(cmd *Command, _ []string) error {
	if dagsDir, _ := cmd.cmd.Flags().GetString("dags"); dagsDir != "" {
		cmd.cfg.Paths.DAGsDir = dagsDir
	}

	ctx := cmd.ctx
	logger.Info(ctx, "Scheduler initialization", "specsDirectory", cmd.cfg.Paths.DAGsDir, "logFormat", cmd.cfg.Global.LogFormat)

	scheduler, err := cmd.scheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler in directory %s: %w",
			cmd.cfg.Paths.DAGsDir, err)
	}

	return nil
}
