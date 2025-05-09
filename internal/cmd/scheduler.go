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
			Long: `Launch the scheduler process that monitors and triggers workflows based on cron schedules.

Example:
  dagu scheduler --dags=/path/to/dags

This process runs continuously to automatically execute scheduled DAGs.
`,
		}, schedulerFlags, runScheduler,
	)
}

var schedulerFlags = []commandLineFlag{dagsFlag}

func runScheduler(ctx *Context, _ []string) error {
	if dagsDir, _ := ctx.Command.Flags().GetString("dags"); dagsDir != "" {
		ctx.Config.Paths.DAGsDir = dagsDir
	}

	logger.Info(ctx, "Scheduler initialization", "specsDirectory", ctx.Config.Paths.DAGsDir, "logFormat", ctx.Config.Global.LogFormat)

	scheduler, err := ctx.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	if err := scheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start scheduler in directory %s: %w",
			ctx.Config.Paths.DAGsDir, err)
	}

	return nil
}
