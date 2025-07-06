package cmd

import (
	"fmt"
	"strconv"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdCoordinator() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "coordinator [flags]",
			Short: "Start the coordinator gRPC server for distributed task execution",
			Long: `Launch the coordinator gRPC server that handles distributed task execution.

The coordinator server provides a central point for distributed workers to:
- Poll for tasks to execute
- Fetch DAG definitions (To be implemented)
- Report task execution status (To be implemented)
- Register themselves with the system (to be implemented)

This server uses gRPC for efficient communication with remote workers and
supports authentication via signing keys configured in the system.

Flags:
  --coordinator-host string   Host address to bind the gRPC server to (default: 127.0.0.1)
  --coordinator-port int      Port number for the gRPC server to listen on (default: 50051)

Example:
  dagu coordinator --coordinator-host=0.0.0.0 --coordinator-port=50051

This process runs continuously in the foreground until terminated.
`,
		}, coordinatorFlags, runCoordinator,
	)
}

var coordinatorFlags = []commandLineFlag{coordinatorHostFlag, coordinatorPortFlag}

func runCoordinator(ctx *Context, _ []string) error {
	// Override config with command line flags if explicitly provided
	if ctx.Command.Flags().Changed("coordinator-host") {
		if host, _ := ctx.Command.Flags().GetString("coordinator-host"); host != "" {
			ctx.Config.Coordinator.Host = host
		}
	}
	if ctx.Command.Flags().Changed("coordinator-port") {
		if portStr, _ := ctx.Command.Flags().GetString("coordinator-port"); portStr != "" {
			// Convert string port to int
			port, err := strconv.Atoi(portStr)
			if err == nil {
				ctx.Config.Coordinator.Port = port
			}
		}
	}

	logger.Info(ctx, "Coordinator initialization", "host", ctx.Config.Coordinator.Host, "port", ctx.Config.Coordinator.Port)

	coordinator, err := ctx.NewCoordinator()
	if err != nil {
		return fmt.Errorf("failed to initialize coordinator: %w", err)
	}

	if err := coordinator.Start(ctx); err != nil {
		return fmt.Errorf("failed to start coordinator: %w", err)
	}

	// Wait for context cancellation
	<-ctx.Done()
	logger.Info(ctx, "Coordinator shutting down")

	if err := coordinator.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop coordinator: %w", err)
	}

	return nil
}
