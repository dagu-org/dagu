package cmd

import (
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/spf13/cobra"
)

func StartAll() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start-all [flags]",
			Short: "Launch web UI server, scheduler, and optionally coordinator in a single process",
			Long: `Simultaneously start the web UI server, scheduler, and optionally coordinator in a single command.

This convenience command combines the functionality of 'dagu server', 'dagu scheduler',
and optionally 'dagu coordinator' into a single process. The web UI provides the management
interface, the scheduler handles automated DAG-run execution based on defined schedules,
and the coordinator (when enabled) manages distributed task execution across workers.

By default, start-all runs in single instance mode without the coordinator. The coordinator
is only started when --coordinator.host is set to a non-localhost address (not 127.0.0.1
or localhost), enabling distributed execution mode.

Flags:
  --host string                     Host address to bind the web server to (default: 127.0.0.1)
  --port int                        Port number for the web server to listen on (default: 8080)
  --dags string                     Path to the directory containing DAG definition files
  --coordinator.host string         Host address to bind the coordinator gRPC server to (default: 127.0.0.1)
  --coordinator.advertise string    Address to advertise in service registry (default: auto-detected hostname)
  --coordinator.port int            Port number for the coordinator gRPC server (default: 50055)
  --peer.cert-file string           Path to TLS certificate file for peer connections
  --peer.key-file string            Path to TLS key file for peer connections
  --peer.client-ca-file string      Path to CA certificate file for client verification (mTLS)
  --peer.insecure                   Use insecure connection (h2c) instead of TLS
  --peer.skip-tls-verify            Skip TLS certificate verification (insecure)

Example:
  # Single instance mode (coordinator disabled)
  dagu start-all

  # Distributed mode (coordinator enabled)
  dagu start-all --coordinator.host=0.0.0.0 --coordinator.port=50055

  # Production with both web and coordinator on all interfaces
  dagu start-all --host=0.0.0.0 --port=8080 --coordinator.host=0.0.0.0

This process runs continuously in the foreground until terminated.
`,
		}, startAllFlags, runStartAll,
	)
}

var startAllFlags = []commandLineFlag{
	dagsFlag,
	hostFlag,
	portFlag,
	coordinatorHostFlag,
	coordinatorPortFlag,
	coordinatorAdvertiseFlag,
	// Peer configuration flags for TLS
	peerInsecureFlag,
	peerCertFileFlag,
	peerKeyFileFlag,
	peerClientCAFileFlag,
	peerSkipTLSVerifyFlag,
}

func runStartAll(ctx *Context, _ []string) error {
	if dagsDir, _ := ctx.Command.Flags().GetString("dags"); dagsDir != "" {
		ctx.Config.Paths.DAGsDir = dagsDir
	}

	// Create a context that will be cancelled on interrupt signal
	signalCtx, stop := signal.NotifyContext(ctx.Context, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Initialize all services
	scheduler, err := ctx.NewScheduler()
	if err != nil {
		return fmt.Errorf("failed to initialize scheduler: %w", err)
	}
	// Disable health server when running from start-all
	scheduler.DisableHealthServer()

	server, err := ctx.NewServer()
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	// Only start coordinator if not bound to localhost
	var coordinator *coordinator.Service
	enableCoordinator := ctx.Config.Coordinator.Host != "127.0.0.1" && ctx.Config.Coordinator.Host != "localhost" && ctx.Config.Coordinator.Host != "::1"

	if enableCoordinator {
		coordinator, err = newCoordinator(ctx, ctx.Config, ctx.ServiceRegistry)
		if err != nil {
			return fmt.Errorf("failed to initialize coordinator: %w", err)
		}
	} else {
		logger.Info(ctx, "Coordinator disabled (bound to localhost), set --coordinator.host and --coordinator.advertise to enable distributed mode")
	}

	// Create a new context with the signal context for services
	serviceCtx := &Context{
		Context:         signalCtx,
		Command:         ctx.Command,
		Flags:           ctx.Flags,
		Config:          ctx.Config,
		Quiet:           ctx.Quiet,
		DAGRunStore:     ctx.DAGRunStore,
		DAGRunMgr:       ctx.DAGRunMgr,
		ProcStore:       ctx.ProcStore,
		QueueStore:      ctx.QueueStore,
		ServiceRegistry: ctx.ServiceRegistry,
	}

	// WaitGroup to track all services
	var wg sync.WaitGroup
	serviceCount := 2 // scheduler + server
	if enableCoordinator {
		serviceCount = 3 // + coordinator
	}
	errCh := make(chan error, serviceCount)

	// Start scheduler
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(serviceCtx, "Scheduler initialization", tag.Dir, serviceCtx.Config.Paths.DAGsDir)
		if err := scheduler.Start(serviceCtx); err != nil {
			select {
			case errCh <- fmt.Errorf("scheduler failed: %w", err):
			default:
			}
		}
	}()

	// Start coordinator (if enabled)
	if enableCoordinator {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := coordinator.Start(serviceCtx); err != nil {
				select {
				case errCh <- fmt.Errorf("coordinator failed: %w", err):
				default:
				}
			}
		}()
	}

	// Start server
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give scheduler and coordinator a moment to start
		time.Sleep(100 * time.Millisecond)
		logger.Info(serviceCtx, "Server initialization", tag.Host, serviceCtx.Config.Server.Host, tag.Port, serviceCtx.Config.Server.Port)
		if err := server.Serve(serviceCtx); err != nil {
			select {
			case errCh <- fmt.Errorf("server failed: %w", err):
			default:
			}
		}
	}()

	// Wait for signal or error
	var firstErr error
	select {
	case <-signalCtx.Done():
		logger.Info(ctx, "Received shutdown signal", tag.Signal, signalCtx.Err())
	case err := <-errCh:
		firstErr = err
		logger.Error(ctx, "Service failed, shutting down", tag.Error, err)
		stop() // Cancel the signal context to trigger shutdown of other services
	}

	// Stop all services gracefully
	logger.Info(ctx, "Stopping all services")

	// Stop coordinator first to unregister from service registry (if it was started)
	if enableCoordinator && coordinator != nil {
		if err := coordinator.Stop(ctx); err != nil {
			logger.Error(ctx, "Failed to stop coordinator", tag.Error, err)
		}
	}

	// Wait for all services to finish with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		logger.Info(ctx, "All services stopped gracefully")
	case <-time.After(30 * time.Second):
		logger.Error(ctx, "Timeout waiting for services to stop")
	}

	return firstErr
}
