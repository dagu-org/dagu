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
  --worker.id string                       Worker instance ID (default: hostname@PID)
  --worker.max-active-runs int             Maximum number of active runs (default: 100)
  --worker.coordinator-host string         Coordinator gRPC server host (default: 127.0.0.1)
  --worker.coordinator-port int            Coordinator gRPC server port (default: 50051)
  --worker.insecure                        Use insecure connection (h2c) instead of TLS (default: true)
  --worker.tls-cert string                 Path to TLS certificate file for mutual TLS
  --worker.tls-key string                  Path to TLS key file for mutual TLS
  --worker.tls-ca string                   Path to CA certificate file for server verification
  --worker.skip-tls-verify                 Skip TLS certificate verification (insecure)
  --worker.labels -l string                Worker labels for capability matching (format: key1=value1,key2=value2)

Example:
  dagu worker
  dagu worker --worker.max-active-runs=50
  dagu worker --worker.coordinator-host=coordinator.example.com --worker.coordinator-port=50051
  dagu worker --worker.id=worker-1 --worker.max-active-runs=200
  
  # Worker with labels for capability matching:
  dagu worker --worker.labels gpu=true,memory=64G,region=us-east-1
  dagu worker --worker.labels cpu-arch=amd64,instance-type=m5.xlarge
  
  # For TLS connections (when coordinator has TLS enabled):
  dagu worker --worker.insecure=false --worker.coordinator-host=coordinator.example.com
  dagu worker --worker.insecure=false --worker.tls-cert=client.crt --worker.tls-key=client.key
  dagu worker --worker.insecure=false --worker.tls-ca=ca.crt
  dagu worker --worker.insecure=false --worker.skip-tls-verify  # For self-signed certificates

This process runs continuously in the foreground until terminated.
`,
		}, workerFlags, runWorker,
	)
}

var workerFlags = []commandLineFlag{
	workerIDFlag,
	workerMaxActiveRunsFlag,
	workerCoordinatorHostFlag,
	workerCoordinatorPortFlag,
	workerInsecureFlag,
	workerTLSCertFlag,
	workerTLSKeyFlag,
	workerTLSCAFlag,
	workerSkipTLSVerifyFlag,
	workerLabelsFlag,
}

func runWorker(ctx *Context, _ []string) error {
	// Use config values directly - viper binding handles flag overrides
	workerID := ctx.Config.Worker.ID
	maxActiveRuns := ctx.Config.Worker.MaxActiveRuns
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
	labels := ctx.Config.Worker.Labels
	if labels == nil {
		labels = make(map[string]string)
	}
	w := worker.NewWorker(workerID, maxActiveRuns, coordinatorHost, coordinatorPort, tlsConfig, ctx.DAGRunMgr, labels)

	logger.Info(ctx, "Starting worker",
		"worker_id", workerID,
		"max_active_runs", maxActiveRuns,
		"coordinator_host", coordinatorHost,
		"coordinator_port", coordinatorPort,
		"labels", labels)

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
