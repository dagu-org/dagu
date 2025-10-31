package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/spf13/cobra"
)

func Scheduler() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "scheduler [flags]",
			Short: "Start the scheduler for automated DAG-run execution",
			Long: `Launch the scheduler process that monitors DAG definitions and automatically triggers DAG based on their defined schedules.

The scheduler continuously monitors the specified directory for DAG definitions,
evaluates their schedule expressions (cron format), and initiates DAG-run executions
when their scheduled time arrives. It also consumes DAG-runs from the queue and executes them.

Flags:
  --dags string   Path to the directory containing DAG definition files

Example:
  dagu scheduler --dags=/path/to/dags

This process runs continuously in the foreground until terminated.
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
