package cmd

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/service/coordinator"
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
  --worker.coordinators string             Coordinator addresses for static discovery (format: host1:port1,host2:port2)

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

  # Shared-nothing deployment (worker doesn't need shared filesystem):
  dagu worker --worker.coordinators=coordinator-1:50055,coordinator-2:50055

This process runs continuously in the foreground until terminated.
`,
		}, workerFlags, runWorker,
	)
}

var workerFlags = []commandLineFlag{
	workerIDFlag,
	workerMaxActiveRunsFlag,
	workerLabelsFlag,
	workerCoordinatorsFlag,
	// Peer configuration flags for TLS
	peerInsecureFlag,
	peerCertFileFlag,
	peerKeyFileFlag,
	peerClientCAFileFlag,
	peerSkipTLSVerifyFlag,
}

func runWorker(ctx *Context, _ []string) error {
	workerID := ctx.Config.Worker.ID
	// Default to hostname@PID if not configured
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil || hostname == "" {
			hostname = "unknown"
		}
		workerID = fmt.Sprintf("%s@%d", hostname, os.Getpid())
	}

	maxActiveRuns := ctx.Config.Worker.MaxActiveRuns
	labels := ctx.Config.Worker.Labels

	coordinatorCli, useRemoteHandler, err := createCoordinatorClient(ctx)
	if err != nil {
		return err
	}

	w := worker.NewWorker(workerID, maxActiveRuns, coordinatorCli, labels, ctx.Config)

	if useRemoteHandler {
		handlerCfg := worker.RemoteTaskHandlerConfig{
			WorkerID:          workerID,
			CoordinatorClient: coordinatorCli,
			PeerConfig:        ctx.Config.Core.Peer,
			Config:            ctx.Config,
		}
		w.SetHandler(worker.NewRemoteTaskHandler(handlerCfg))
		logger.Info(ctx, "Using remote task handler for shared-nothing mode")
	}

	logger.Info(ctx, "Starting worker", tag.WorkerID(workerID), tag.MaxConcurrency(maxActiveRuns), slog.Any("labels", labels))

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

// BuildCoordinatorClientConfig creates coordinator client config from application config.
// Returns nil config if no static coordinators configured (use service registry instead).
// This is a pure function that can be unit tested without network I/O.
func BuildCoordinatorClientConfig(cfg *config.Config) (*coordinator.Config, bool, error) {
	if len(cfg.Worker.Coordinators) == 0 {
		return nil, false, nil // Use service registry discovery
	}

	coordinatorCliCfg := coordinator.DefaultConfig()
	coordinatorCliCfg.CAFile = cfg.Core.Peer.ClientCaFile
	coordinatorCliCfg.CertFile = cfg.Core.Peer.CertFile
	coordinatorCliCfg.KeyFile = cfg.Core.Peer.KeyFile
	coordinatorCliCfg.SkipTLSVerify = cfg.Core.Peer.SkipTLSVerify
	coordinatorCliCfg.Insecure = cfg.Core.Peer.Insecure

	if err := coordinatorCliCfg.Validate(); err != nil {
		return nil, false, fmt.Errorf("invalid coordinator client configuration: %w", err)
	}

	return coordinatorCliCfg, true, nil
}

// createCoordinatorClient creates the appropriate coordinator client based on configuration.
// Returns the client, whether to use remote handler, and any error.
func createCoordinatorClient(ctx *Context) (coordinator.Client, bool, error) {
	coordinatorCliCfg, useRemoteHandler, err := BuildCoordinatorClientConfig(ctx.Config)
	if err != nil {
		return nil, false, err
	}

	if coordinatorCliCfg == nil {
		return ctx.NewCoordinatorClient(), false, nil
	}

	staticRegistry, err := coordinator.NewStaticRegistry(ctx.Config.Worker.Coordinators)
	if err != nil {
		return nil, false, fmt.Errorf("failed to create static registry: %w", err)
	}
	logger.Info(ctx, "Using static coordinator discovery",
		slog.Any("coordinators", ctx.Config.Worker.Coordinators))

	return coordinator.New(staticRegistry, coordinatorCliCfg), useRemoteHandler, nil
}
