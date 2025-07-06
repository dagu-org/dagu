package cmd

import (
	"fmt"
	"strconv"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/worker"
	"github.com/spf13/cobra"
)

func CmdWorker() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "worker [flags]",
			Short: "Start a worker that polls the coordinator for tasks",
			Long: `Launch a worker process that connects to the coordinator and polls for tasks.

The worker creates multiple concurrent pollers (goroutines) that continuously
poll the coordinator for tasks to execute. Each poller generates a unique
poller_id for every poll request.

By default, the worker ID is set to hostname@PID, but can be overridden.

Flags:
  --worker-id string              Worker instance ID (default: hostname@PID)
  --max-concurrent-runs int       Maximum concurrent task executions (default: 100)
  --coordinator-host string       Coordinator gRPC server host (default: 127.0.0.1)
  --coordinator-port int          Coordinator gRPC server port (default: 50051)
  --coordinator-insecure          Use insecure connection (h2c) instead of TLS
  --coordinator-tls-cert string   Path to TLS certificate file for mutual TLS
  --coordinator-tls-key string    Path to TLS key file for mutual TLS
  --coordinator-tls-ca string     Path to CA certificate file for server verification
  --coordinator-skip-tls-verify   Skip TLS certificate verification (insecure)

Example:
  dagu worker
  dagu worker --max-concurrent-runs=50
  dagu worker --coordinator-host=coordinator.example.com --coordinator-port=50051
  dagu worker --worker-id=worker-1 --max-concurrent-runs=200
  
  # For HTTPS/TLS connections:
  dagu worker --coordinator-host=coordinator.example.com
  dagu worker --coordinator-tls-cert=client.crt --coordinator-tls-key=client.key
  dagu worker --coordinator-tls-ca=ca.crt
  dagu worker --coordinator-skip-tls-verify  # For self-signed certificates
  dagu worker --coordinator-insecure         # For h2c (HTTP/2 without TLS)

This process runs continuously in the foreground until terminated.
`,
		}, workerFlags, runWorker,
	)
}

var workerFlags = []commandLineFlag{
	workerIDFlag,
	maxConcurrentRunsFlag,
	coordinatorHostFlag,
	coordinatorPortFlag,
	coordinatorInsecureFlag,
	coordinatorTLSCertFlag,
	coordinatorTLSKeyFlag,
	coordinatorTLSCAFlag,
	coordinatorSkipTLSVerifyFlag,
}

func runWorker(ctx *Context, _ []string) error {
	// Get worker ID (optional, defaults to hostname@PID)
	workerID, _ := ctx.Command.Flags().GetString("worker-id")

	// Get max concurrent runs
	maxConcurrentRunsStr, _ := ctx.Command.Flags().GetString("max-concurrent-runs")
	maxConcurrentRuns, err := strconv.Atoi(maxConcurrentRunsStr)
	if err != nil {
		return fmt.Errorf("invalid max-concurrent-runs value: %w", err)
	}

	// Override config with command line flags if explicitly provided
	coordinatorHost := ctx.Config.Coordinator.Host
	coordinatorPort := ctx.Config.Coordinator.Port

	if ctx.Command.Flags().Changed("coordinator-host") {
		if host, _ := ctx.Command.Flags().GetString("coordinator-host"); host != "" {
			coordinatorHost = host
		}
	}
	if ctx.Command.Flags().Changed("coordinator-port") {
		if portStr, _ := ctx.Command.Flags().GetString("coordinator-port"); portStr != "" {
			port, err := strconv.Atoi(portStr)
			if err == nil {
				coordinatorPort = port
			}
		}
	}

	// Build TLS configuration
	tlsConfig := &worker.TLSConfig{}

	// Check if insecure flag is set
	if insecure, _ := ctx.Command.Flags().GetBool("coordinator-insecure"); insecure {
		tlsConfig.Insecure = true
	}

	// Get TLS certificate files
	tlsConfig.CertFile, _ = ctx.Command.Flags().GetString("coordinator-tls-cert")
	tlsConfig.KeyFile, _ = ctx.Command.Flags().GetString("coordinator-tls-key")
	tlsConfig.CAFile, _ = ctx.Command.Flags().GetString("coordinator-tls-ca")

	// Check skip TLS verify flag
	if skipVerify, _ := ctx.Command.Flags().GetBool("coordinator-skip-tls-verify"); skipVerify {
		tlsConfig.SkipTLSVerify = true
	}

	// Create and start the worker
	w := worker.NewWorker(workerID, maxConcurrentRuns, coordinatorHost, coordinatorPort, tlsConfig)

	logger.Info(ctx, "Starting worker",
		"worker_id", workerID,
		"max_concurrent_runs", maxConcurrentRuns,
		"coordinator_host", coordinatorHost,
		"coordinator_port", coordinatorPort)

	// Start the worker in a goroutine to allow for graceful shutdown
	errCh := make(chan error, 1)
	go func() {
		if err := w.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	// Wait for either context cancellation or an error
	select {
	case <-ctx.Done():
		logger.Info(ctx, "Worker shutting down")
		if err := w.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop worker: %w", err)
		}
	case err := <-errCh:
		return fmt.Errorf("worker failed: %w", err)
	}

	return nil
}
