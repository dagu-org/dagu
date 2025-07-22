package cmd

import (
	"fmt"

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
supports authentication via signing keys and TLS encryption.

Flags:
  --coordinator.host string         Host address to bind the gRPC server to (default: 127.0.0.1)
  --coordinator.port int            Port number for the gRPC server to listen on (default: 50055)
  --coordinator.signing-key string  Signing key for coordinator authentication
  --coordinator.tls-cert string     Path to TLS certificate file for the coordinator server
  --coordinator.tls-key string      Path to TLS key file for the coordinator server
  --coordinator.tls-ca string       Path to CA certificate file for client verification (mTLS)

Example:
  # Basic usage
  dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055

  # With authentication
  dagu coordinator --coordinator.signing-key=mysecretkey

  # With TLS
  dagu coordinator --coordinator.tls-cert=server.crt --coordinator.tls-key=server.key

  # With mutual TLS
  dagu coordinator --coordinator.tls-cert=server.crt --coordinator.tls-key=server.key --coordinator.tls-ca=ca.crt

This process runs continuously in the foreground until terminated.
`,
		}, coordinatorFlags, runCoordinator,
	)
}

var coordinatorFlags = []commandLineFlag{
	coordinatorHostFlag,
	coordinatorPortFlag,
	coordinatorSigningKeyFlag,
	coordinatorTLSCertFlag,
	coordinatorTLSKeyFlag,
	coordinatorTLSCAFlag,
}

func runCoordinator(ctx *Context, _ []string) error {
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
