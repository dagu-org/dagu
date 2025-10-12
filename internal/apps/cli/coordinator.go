package cli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"os"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/core/execution"
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
  --peer.cert-file string           Path to TLS certificate file for peer connections
  --peer.key-file string            Path to TLS key file for peer connections
  --peer.client-ca-file string      Path to CA certificate file for client verification (mTLS)
  --peer.insecure                   Use insecure connection (h2c) instead of TLS
  --peer.skip-tls-verify            Skip TLS certificate verification (insecure)

Example:
  # Basic usage
  dagu coordinator --coordinator.host=0.0.0.0 --coordinator.port=50055

  # With TLS
  dagu coordinator --peer.cert-file=server.crt --peer.key-file=server.key

  # With mutual TLS
  dagu coordinator --peer.cert-file=server.crt --peer.key-file=server.key --peer.client-ca-file=ca.crt

This process runs continuously in the foreground until terminated.
`,
		}, coordinatorFlags, runCoordinator,
	)
}

var coordinatorFlags = []commandLineFlag{
	coordinatorHostFlag,
	coordinatorPortFlag,
	// Peer configuration flags for TLS
	peerInsecureFlag,
	peerCertFileFlag,
	peerKeyFileFlag,
	peerClientCAFileFlag,
	peerSkipTLSVerifyFlag,
}

func runCoordinator(ctx *Context, _ []string) error {
	coordinator, err := newCoordinator(ctx, ctx.Config, ctx.ServiceRegistry)
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
func newCoordinator(ctx context.Context, cfg *config.Config, registry execution.ServiceRegistry) (*coordinator.Service, error) {
	// Generate instance ID
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}
	instanceID := fmt.Sprintf("%s@%d", hostname, cfg.Coordinator.Port)

	logger.Info(ctx, "Coordinator initialization",
		"host", cfg.Coordinator.Host,
		"port", cfg.Coordinator.Port,
		"instance_id", instanceID)
	// Create gRPC server options
	var serverOpts []grpc.ServerOption

	// Configure TLS using global peer config
	if cfg.Global.Peer.CertFile != "" && cfg.Global.Peer.KeyFile != "" {
		// Load server certificates
		creds, err := loadCoordinatorTLSCredentials(&config.TLSConfig{
			CertFile: cfg.Global.Peer.CertFile,
			KeyFile:  cfg.Global.Peer.KeyFile,
			CAFile:   cfg.Global.Peer.ClientCaFile,
		})
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
	addr := fmt.Sprintf("%s:%d", cfg.Coordinator.Host, cfg.Coordinator.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener on %s: %w", addr, err)
	}

	// Create handler
	handler := coordinator.NewHandler()

	// Create and return service
	return coordinator.NewService(grpcServer, handler, listener, healthServer, registry, instanceID, cfg.Coordinator.Host), nil
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
