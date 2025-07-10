package cmd

import (
	"fmt"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/spf13/cobra"
)

func CmdStartAll() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "start-all [flags]",
			Short: "Launch web UI server, scheduler, and coordinator in a single process",
			Long: `Simultaneously start the web UI server, scheduler, and coordinator in a single command.

This convenience command combines the functionality of 'dagu server', 'dagu scheduler',
and 'dagu coordinator' into a single process, making it easier to run a complete Dagu
instance with distributed execution capabilities. The web UI provides the management
interface, the scheduler handles automated DAG-run execution based on defined schedules,
and the coordinator manages distributed task execution across workers.

Flags:
  --host string                     Host address to bind the web server to (default: 127.0.0.1)
  --port int                        Port number for the web server to listen on (default: 8080)
  --dags string                     Path to the directory containing DAG definition files
  --coordinator-host string         Host address to bind the coordinator gRPC server to (default: 127.0.0.1)
  --coordinator-port int            Port number for the coordinator gRPC server (default: 50051)
  --coordinator-signing-key string  Signing key for coordinator authentication
  --coordinator-tls-cert string     Path to TLS certificate file for the coordinator server
  --coordinator-tls-key string      Path to TLS key file for the coordinator server
  --coordinator-tls-ca string       Path to CA certificate file for client verification (mTLS)

Example:
  dagu start-all --host=0.0.0.0 --port=8080 --dags=/path/to/dags --coordinator-port=50051

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
	coordinatorSigningKeyFlag,
	coordinatorTLSCertFlag,
	coordinatorTLSKeyFlag,
	coordinatorTLSCAFlag,
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

	server, err := ctx.NewServer()
	if err != nil {
		return fmt.Errorf("failed to initialize server: %w", err)
	}

	coordinator, err := ctx.NewCoordinator()
	if err != nil {
		return fmt.Errorf("failed to initialize coordinator: %w", err)
	}

	// Create a new context with the signal context for services
	serviceCtx := &Context{
		Context:     signalCtx,
		Command:     ctx.Command,
		Flags:       ctx.Flags,
		Config:      ctx.Config,
		Quiet:       ctx.Quiet,
		DAGRunStore: ctx.DAGRunStore,
		DAGRunMgr:   ctx.DAGRunMgr,
		ProcStore:   ctx.ProcStore,
		QueueStore:  ctx.QueueStore,
	}

	// WaitGroup to track all services
	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	// Start scheduler
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(serviceCtx, "Scheduler initialization", "dags", serviceCtx.Config.Paths.DAGsDir)
		if err := scheduler.Start(serviceCtx); err != nil {
			select {
			case errCh <- fmt.Errorf("scheduler failed: %w", err):
			default:
			}
		}
	}()

	// Start coordinator
	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info(serviceCtx, "Coordinator initialization", "host", serviceCtx.Config.Coordinator.Host, "port", serviceCtx.Config.Coordinator.Port)
		if err := coordinator.Start(serviceCtx); err != nil {
			select {
			case errCh <- fmt.Errorf("coordinator failed: %w", err):
			default:
			}
		}
	}()

	// Start server
	wg.Add(1)
	go func() {
		defer wg.Done()
		// Give scheduler and coordinator a moment to start
		time.Sleep(100 * time.Millisecond)
		logger.Info(serviceCtx, "Server initialization", "host", serviceCtx.Config.Server.Host, "port", serviceCtx.Config.Server.Port)
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
		logger.Info(ctx, "Received shutdown signal", "signal", signalCtx.Err())
	case err := <-errCh:
		firstErr = err
		logger.Error(ctx, "Service failed, shutting down", "err", err)
		stop() // Cancel the signal context to trigger shutdown of other services
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
