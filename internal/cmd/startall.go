package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStartAll() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start-all [flags]",
			Short: "Launch both web UI server and scheduler in a single process",
			Long: `Simultaneously start both the web UI server and the scheduler process in a single command.

This convenience command combines the functionality of 'dagu server' and 'dagu scheduler'
into a single process, making it easier to run a complete Dagu instance. The web UI
provides the management interface while the scheduler handles automated DAG-run execution
based on defined schedules.

Flags:
  --host string    Host address to bind the web server to (default: 127.0.0.1)
  --port int       Port number for the web server to listen on (default: 8080)
  --dags string    Path to the directory containing DAG definition files

Example:
  dagu start-all --host=0.0.0.0 --port=8080 --dags=/path/to/dags

This process runs continuously in the foreground until terminated.
`,
		}, startAllFlags, runStartAll,
	)
}

var startAllFlags = []commandLineFlag{dagsFlag, hostFlag, portFlag}

func runStartAll(ctx *Context, _ []string) error {
	if dagsDir, _ := ctx.Command.Flags().GetString("dags"); dagsDir != "" {
		ctx.Config.Paths.DAGsDir = dagsDir
	}

	// Run auto-migration if needed
	if err := AutoMigrate(ctx); err != nil {
		logger.Error(ctx, "Failed to run auto-migration", "error", err)
		// Continue with startup even if migration fails
	}

	scheduler, err := ctx.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	// Start scheduler in a goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info(ctx, "Scheduler initialization", "dags", ctx.Config.Paths.DAGsDir)

		if err := scheduler.Start(ctx); err != nil {
			errChan <- fmt.Errorf("scheduler initialization failed: %w", err)
			return
		}
		errChan <- nil
	}()

	server, err := ctx.NewServer()
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Start server in a goroutine
	logger.Info(ctx, "Server initialization", "host", ctx.Config.Server.Host, "port", ctx.Config.Server.Port)

	serverErr := make(chan error, 1)
	go func() {
		if err := server.Serve(ctx); err != nil {
			serverErr <- fmt.Errorf("server initialization failed: %w", err)
			return
		}
		serverErr <- nil
	}()

	// Wait for either error to occur
	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
	case err := <-serverErr:
		if err != nil {
			return err
		}
	case <-ctx.Done():
		logger.Info(ctx, "Context cancelled")
		return nil
	}

	return nil
}
