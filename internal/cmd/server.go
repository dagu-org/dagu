package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdServer() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server [flags]",
		Short: "Start the web server",
		Long: `Launch the Dagu web server, providing a real-time graphical interface for monitoring and managing DAG executions.

Example:
  dagu server --host=0.0.0.0 --port=8080 --dags=/path/to/dags
`,
		RunE: wrapRunE(runServer),
	}
	initFlags(cmd, serverFlags...)
	return cmd
}

var serverFlags = []commandLineFlag{dagsFlag, hostFlag, portFlag}

func runServer(cmd *cobra.Command, _ []string) error {
	bindFlags(cmd, serverFlags...)

	setup, err := createSetup(cmd, serverFlags, false)
	if err != nil {
		return fmt.Errorf("failed to create setup: %w", err)
	}

	ctx := setup.ctx
	logger.Info(ctx, "Server initialization", "host", setup.cfg.Server.Host, "port", setup.cfg.Server.Port)

	server, err := setup.server(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
