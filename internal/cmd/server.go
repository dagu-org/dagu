package cmd

import (
	"fmt"
	"os/signal"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/service/resource"
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

// runServer initializes and runs the web UI server and its resource monitoring service.
// It logs startup info, starts the resource service (deferring its shutdown and logging any stop errors),
// constructs the server with that resource service, and then begins serving.
// It returns an error if the resource service fails to start, the server fails to initialize, or serving fails.
func runServer(ctx *Context, _ []string) error {
	// Create a context that will be cancelled on interrupt signal.
	// This must be created BEFORE server initialization so OIDC provider init can be cancelled.
	signalCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create a signal-aware context for services
	serviceCtx := ctx.WithContext(signalCtx)

	logger.Info(serviceCtx, "Server initialization",
		tag.Host(serviceCtx.Config.Server.Host),
		tag.Port(serviceCtx.Config.Server.Port),
	)

	// Initialize resource monitoring service (defer cleanup, but don't start yet).
	// Resource monitoring must start AFTER server init to avoid race condition
	// with OIDC provider initialization (gopsutil conflicts with net/http dial).
	resourceService := resource.NewService(ctx.Config)
	defer func() {
		if err := resourceService.Stop(ctx); err != nil {
			logger.Error(ctx, "Failed to stop resource service", tag.Error(err))
		}
	}()

	// Initialize server (includes OIDC setup). Use serviceCtx so OIDC can
	// respond to termination signals during potentially slow network operations.
	server, err := serviceCtx.NewServer(resourceService)
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Start resource monitoring now that server initialization is complete.
	if err := resourceService.Start(serviceCtx); err != nil {
		return fmt.Errorf("failed to start resource service: %w", err)
	}

	if err := server.Serve(serviceCtx); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
