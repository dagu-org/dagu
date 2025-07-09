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
	// Use config values as defaults, command-line flags override
	workerID := ctx.Config.Worker.ID
	if id, _ := ctx.Command.Flags().GetString("worker-id"); id != "" && ctx.Command.Flags().Changed("worker-id") {
		workerID = id
	}

	// Get max concurrent runs
	maxConcurrentRuns := ctx.Config.Worker.MaxConcurrentRuns
	if ctx.Command.Flags().Changed("max-concurrent-runs") {
		if str, _ := ctx.Command.Flags().GetString("max-concurrent-runs"); str != "" {
			if val, err := strconv.Atoi(str); err == nil {
				maxConcurrentRuns = val
			} else {
				return fmt.Errorf("invalid max-concurrent-runs value: %w", err)
			}
		}
	}

	// Use worker config for coordinator connection
	coordinatorHost := ctx.Config.Worker.CoordinatorHost
	if ctx.Command.Flags().Changed("coordinator-host") {
		if host, _ := ctx.Command.Flags().GetString("coordinator-host"); host != "" {
			coordinatorHost = host
		}
	}

	coordinatorPort := ctx.Config.Worker.CoordinatorPort
	if ctx.Command.Flags().Changed("coordinator-port") {
		if portStr, _ := ctx.Command.Flags().GetString("coordinator-port"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil {
				coordinatorPort = port
			}
		}
	}

	// Build TLS configuration from config, then override with flags
	tlsConfig := &worker.TLSConfig{
		Insecure:      ctx.Config.Worker.Insecure,
		SkipTLSVerify: ctx.Config.Worker.SkipTLSVerify,
	}

	if ctx.Config.Worker.TLS != nil {
		tlsConfig.CertFile = ctx.Config.Worker.TLS.CertFile
		tlsConfig.KeyFile = ctx.Config.Worker.TLS.KeyFile
		tlsConfig.CAFile = ctx.Config.Worker.TLS.CAFile
	}

	// Command-line flags override config
	if ctx.Command.Flags().Changed("coordinator-insecure") {
		tlsConfig.Insecure, _ = ctx.Command.Flags().GetBool("coordinator-insecure")
	}
	if ctx.Command.Flags().Changed("coordinator-skip-tls-verify") {
		tlsConfig.SkipTLSVerify, _ = ctx.Command.Flags().GetBool("coordinator-skip-tls-verify")
	}
	if cert, _ := ctx.Command.Flags().GetString("coordinator-tls-cert"); cert != "" && ctx.Command.Flags().Changed("coordinator-tls-cert") {
		tlsConfig.CertFile = cert
	}
	if key, _ := ctx.Command.Flags().GetString("coordinator-tls-key"); key != "" && ctx.Command.Flags().Changed("coordinator-tls-key") {
		tlsConfig.KeyFile = key
	}
	if ca, _ := ctx.Command.Flags().GetString("coordinator-tls-ca"); ca != "" && ctx.Command.Flags().Changed("coordinator-tls-ca") {
		tlsConfig.CAFile = ca
	}

	// Create and start the worker
	w := worker.NewWorker(workerID, maxConcurrentRuns, coordinatorHost, coordinatorPort, tlsConfig, ctx.DAGRunMgr)

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
