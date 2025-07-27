package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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
  --coordinator.tls-cert string     Path to TLS certificate file for the coordinator server
  --coordinator.tls-key string      Path to TLS key file for the coordinator server
  --coordinator.tls-ca string       Path to CA certificate file for client verification (mTLS)

Example:
  # Basic usage
  dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055

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
	coordinatorTLSCertFlag,
	coordinatorTLSKeyFlag,
	coordinatorTLSCAFlag,
}

func runCoordinator(ctx *Context, _ []string) error {
	logger.Info(ctx, "Coordinator initialization", "host", ctx.Config.Coordinator.Host, "port", ctx.Config.Coordinator.Port)

	coordinator, err := newCoordinator(ctx.Config.Coordinator)
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

// newCoordinator creates a new Coordinator service instance.
// It sets up a gRPC server and listener for distributed task coordination.
func newCoordinator(cfg config.Coordinator) (*coordinator.Service, error) {
	// Create gRPC server options
	var serverOpts []grpc.ServerOption

	// Configure TLS if enabled
	if cfg.TLS != nil && cfg.TLS.CertFile != "" && cfg.TLS.KeyFile != "" {
		// Load server certificates
		creds, err := loadCoordinatorTLSCredentials(cfg.TLS)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS credentials: %w", err)
		}
		serverOpts = append(serverOpts, grpc.Creds(creds))
	}

	// Create gRPC server with options
	grpcServer := grpc.NewServer(serverOpts...)

	// Create health service
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)

	// Create listener
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on %s: %w", addr, err)
	}

	// Create handler
	handler := coordinator.NewHandler()

	// Create and return service
	return coordinator.NewService(grpcServer, handler, listener, healthServer), nil
}

// loadCoordinatorTLSCredentials loads TLS credentials for the coordinator server.
// It supports both standard TLS and mutual TLS (mTLS) configurations.
func loadCoordinatorTLSCredentials(tlsConfig *config.TLSConfig) (credentials.TransportCredentials, error) {
	if tlsConfig == nil {
		return nil, fmt.Errorf("TLS configuration is nil")
	}

	// Load server certificate and key
	cert, err := tls.LoadX509KeyPair(tlsConfig.CertFile, tlsConfig.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificates: %w", err)
	}

	// Create TLS configuration
	config := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.NoClientCert,
		MinVersion:   tls.VersionTLS12,
	}

	// If CA file is provided, enable mutual TLS
	if tlsConfig.CAFile != "" {
		// Load CA certificate
		caCert, err := os.ReadFile(tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		// Create certificate pool
		certPool := x509.NewCertPool()
		if !certPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		// Update TLS config for mutual TLS
		config.ClientAuth = tls.RequireAndVerifyClientCert
		config.ClientCAs = certPool
	}

	return credentials.NewTLS(config), nil
}
