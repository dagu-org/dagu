package cmd

import (
	"fmt"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/service/worker"
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
  --worker.labels -l string                Worker labels for capability matching (format: key1=value1,key2=value2)

TLS Configuration (uses global peer settings):
  --peer.insecure                          Use insecure connection (h2c) instead of TLS (default: true)
  --peer.cert-file string                  Path to TLS certificate file for mutual TLS
  --peer.key-file string                   Path to TLS key file for mutual TLS
  --peer.client-ca-file string             Path to CA certificate file for server verification
  --peer.skip-tls-verify                   Skip TLS certificate verification (insecure)

Example:
  dagu worker
  dagu worker --worker.max-active-runs=50
  dagu worker --worker.id=worker-1 --worker.max-active-runs=200
  
  # Worker with labels for capability matching:
  dagu worker --worker.labels gpu=true,memory=64G,region=us-east-1
  dagu worker --worker.labels cpu-arch=amd64,instance-type=m5.xlarge
  
  # For TLS connections (when coordinator has TLS enabled):
  dagu worker --peer.insecure=false --peer.cert-file=client.crt --peer.key-file=client.key
  dagu worker --peer.insecure=false --peer.client-ca-file=ca.crt
  dagu worker --peer.insecure=false --peer.skip-tls-verify  # For self-signed certificates

This process runs continuously in the foreground until terminated.
`,
		}, workerFlags, runWorker,
	)
}

var workerFlags = []commandLineFlag{
	workerIDFlag,
	workerMaxActiveRunsFlag,
	workerLabelsFlag,
	// Peer configuration flags for TLS
	peerInsecureFlag,
	peerCertFileFlag,
	peerKeyFileFlag,
	peerClientCAFileFlag,
	peerSkipTLSVerifyFlag,
}

func runWorker(ctx *Context, _ []string) error {
	// Use config values directly - viper binding handles flag overrides
	workerID := ctx.Config.Worker.ID
	maxActiveRuns := ctx.Config.Worker.MaxActiveRuns

	// Create and start the worker
	labels := ctx.Config.Worker.Labels
	if labels == nil {
		labels = make(map[string]string)
	}

	coordinatorCli := ctx.NewCoordinatorClient()
	w := worker.NewWorker(workerID, maxActiveRuns, coordinatorCli, labels, ctx.Config)

	logger.Info(ctx, "Starting worker",
		tag.WorkerID, workerID,
		tag.MaxConcurrency, maxActiveRuns,
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
