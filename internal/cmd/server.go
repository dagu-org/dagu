package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/spf13/cobra"
)

func Server() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "server [flags]",
			Short: "Start the web UI server for DAG management",
			Long: `Launch the Dagu web server that provides a graphical interface for monitoring and managing DAGs.

The web UI allows you to:
- View and manage DAG definitions
- Monitor active and historical DAG-runs
- Inspect DAG-run details including step status and logs
- Start, stop, and retry DAG-runs 
- View execution history and statistics

Flags:
  --host string    Host address to bind the server to (default: 127.0.0.1)
  --port int       Port number to listen on (default: 8080)
  --dags string    Path to the directory containing DAG definition files

Example:
  dagu server --host=0.0.0.0 --port=8080 --dags=/path/to/dags
`,
		}, serverFlags, runServer,
	)
}

var serverFlags = []commandLineFlag{dagsFlag, hostFlag, portFlag}

func runServer(ctx *Context, _ []string) error {
	logger.Info(ctx, "Server initialization",
		tag.Host(ctx.Config.Server.Host),
		tag.Port(ctx.Config.Server.Port),
	)

	server, err := ctx.NewServer()
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
