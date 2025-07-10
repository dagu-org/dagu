package cmd

import (
	"fmt"

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
  --worker-id string                       Worker instance ID (default: hostname@PID)
  --worker-max-concurrent-runs int         Maximum concurrent task executions (default: 100)
  --worker-coordinator-host string         Coordinator gRPC server host (default: 127.0.0.1)
  --worker-coordinator-port int            Coordinator gRPC server port (default: 50051)
  --worker-insecure                        Use insecure connection (h2c) instead of TLS (default: true)
  --worker-tls-cert string                 Path to TLS certificate file for mutual TLS
  --worker-tls-key string                  Path to TLS key file for mutual TLS
  --worker-tls-ca string                   Path to CA certificate file for server verification
  --worker-skip-tls-verify                 Skip TLS certificate verification (insecure)

Example:
  dagu worker
  dagu worker --worker-max-concurrent-runs=50
  dagu worker --worker-coordinator-host=coordinator.example.com --worker-coordinator-port=50051
  dagu worker --worker-id=worker-1 --worker-max-concurrent-runs=200
  
  # For TLS connections (when coordinator has TLS enabled):
  dagu worker --worker-insecure=false --worker-coordinator-host=coordinator.example.com
  dagu worker --worker-insecure=false --worker-tls-cert=client.crt --worker-tls-key=client.key
  dagu worker --worker-insecure=false --worker-tls-ca=ca.crt
  dagu worker --worker-insecure=false --worker-skip-tls-verify  # For self-signed certificates

This process runs continuously in the foreground until terminated.
`,
		}, workerFlags, runWorker,
	)
}

var workerFlags = []commandLineFlag{
	workerIDFlag,
	workerMaxConcurrentRunsFlag,
	workerCoordinatorHostFlag,
	workerCoordinatorPortFlag,
	workerInsecureFlag,
	workerTLSCertFlag,
	workerTLSKeyFlag,
	workerTLSCAFlag,
	workerSkipTLSVerifyFlag,
}

func runWorker(ctx *Context, _ []string) error {
	// Use config values directly - viper binding handles flag overrides
	workerID := ctx.Config.Worker.ID
	maxConcurrentRuns := ctx.Config.Worker.MaxConcurrentRuns
	coordinatorHost := ctx.Config.Worker.CoordinatorHost
	coordinatorPort := ctx.Config.Worker.CoordinatorPort

	// Build TLS configuration from config
	tlsConfig := &worker.TLSConfig{
		Insecure:      ctx.Config.Worker.Insecure,
		SkipTLSVerify: ctx.Config.Worker.SkipTLSVerify,
	}

	if ctx.Config.Worker.TLS != nil {
		tlsConfig.CertFile = ctx.Config.Worker.TLS.CertFile
		tlsConfig.KeyFile = ctx.Config.Worker.TLS.KeyFile
		tlsConfig.CAFile = ctx.Config.Worker.TLS.CAFile
	}

	// Create and start the worker
	// TODO: Add support for configuring worker labels
	labels := make(map[string]string)
	w := worker.NewWorker(workerID, maxConcurrentRuns, coordinatorHost, coordinatorPort, tlsConfig, ctx.DAGRunMgr, labels)

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
