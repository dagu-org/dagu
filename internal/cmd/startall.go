package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStartAll() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start-all [flags]",
		Short: "Launch both web server and scheduler concurrently",
		Long: `Simultaneously start the Dagu web UI server and the scheduler process.

Example:
  dagu start-all --host=0.0.0.0 --port=8080 --dags=/path/to/dags
`,
		RunE: wrapRunE(runStartAll),
	}
	initFlags(cmd, startAllFlags...)
	return cmd
}

var startAllFlags = []commandLineFlag{dagsFlag, hostFlag, portFlag}

func runStartAll(cmd *cobra.Command, _ []string) error {
	setup, err := createSetup(cmd, startAllFlags, false)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	if dagsDir, _ := cmd.Flags().GetString("dags"); dagsDir != "" {
		setup.cfg.Paths.DAGsDir = dagsDir
	}

	scheduler, err := setup.scheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}

	ctx := setup.ctx

	// Start scheduler in a goroutine
	errChan := make(chan error, 1)
	go func() {
		logger.Info(ctx, "Scheduler initialization", "dags", setup.cfg.Paths.DAGsDir)

		if err := scheduler.Start(ctx); err != nil {
			errChan <- fmt.Errorf("scheduler initialization failed: %w", err)
			return
		}
		errChan <- nil
	}()

	server, err := setup.server(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Start server in a goroutine
	logger.Info(ctx, "Server initialization", "host", setup.cfg.Server.Host, "port", setup.cfg.Server.Port)

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
