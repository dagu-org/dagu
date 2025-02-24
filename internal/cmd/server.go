package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdServer() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "server [flags]",
			Short: "Start the web server",
			Long: `Launch the Dagu web server, providing a real-time graphical interface for monitoring and managing DAG executions.

Example:
  dagu server --host=0.0.0.0 --port=8080 --dags=/path/to/dags
`,
		}, serverFlags, runServer,
	)
}

var serverFlags = []commandLineFlag{dagsFlag, hostFlag, portFlag}

func runServer(cmd *Command, _ []string) error {
	ctx := cmd.ctx
	logger.Info(ctx, "Server initialization", "host", cmd.cfg.Server.Host, "port", cmd.cfg.Server.Port)

	server, err := cmd.server(ctx)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
