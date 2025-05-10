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
			Short: "Start the web UI server for workflow management",
			Long: `Launch the Dagu web server that provides a graphical interface for monitoring and managing workflows.

The web UI allows you to:
- View all available DAG definitions
- Monitor active and historical workflow executions
- Inspect workflow details including step status and logs
- Start, stop, and retry workflows
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
	logger.Info(ctx, "Server initialization", "host", ctx.Config.Server.Host, "port", ctx.Config.Server.Port)

	server, err := ctx.NewServer()
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	if err := server.Serve(ctx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
