package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// TLSConfig holds TLS configuration for the worker
type TLSConfig struct {
	Insecure      bool
	CertFile      string
	KeyFile       string
	CAFile        string
	SkipTLSVerify bool
}

// Worker represents a worker instance that polls for tasks from the coordinator.
type Worker struct {
	id                string
	maxConcurrentRuns int
	coordinatorAddr   string
	tlsConfig         *TLSConfig
	client            coordinatorv1.CoordinatorServiceClient
	healthClient      grpc_health_v1.HealthClient
	conn              *grpc.ClientConn
}

// NewWorker creates a new worker instance.
func NewWorker(workerID string, maxConcurrentRuns int, coordinatorHost string, coordinatorPort int, tlsConfig *TLSConfig) *Worker {
	// Generate default worker ID if not provided
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		workerID = fmt.Sprintf("%s@%d", hostname, os.Getpid())
	}

	coordinatorAddr := fmt.Sprintf("%s:%d", coordinatorHost, coordinatorPort)

	return &Worker{
		id:                workerID,
		maxConcurrentRuns: maxConcurrentRuns,
		coordinatorAddr:   coordinatorAddr,
		tlsConfig:         tlsConfig,
	}
}

// Start begins the worker's operation, launching multiple polling goroutines.
func (w *Worker) Start(ctx context.Context) error {
	// Establish gRPC connection with appropriate credentials
	dialOpts, err := w.getDialOptions()
	if err != nil {
		return fmt.Errorf("failed to configure gRPC connection: %w", err)
	}

	conn, err := grpc.NewClient(w.coordinatorAddr, dialOpts...)
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator at %s: %w", w.coordinatorAddr, err)
	}
	w.conn = conn
	w.client = coordinatorv1.NewCoordinatorServiceClient(conn)
	w.healthClient = grpc_health_v1.NewHealthClient(conn)

	logger.Info(ctx, "Worker connected to coordinator",
		"worker_id", w.id,
		"coordinator", w.coordinatorAddr,
		"max_concurrent_runs", w.maxConcurrentRuns)

	// Wait for coordinator to be healthy before starting polling
	if err := w.waitForHealthy(ctx); err != nil {
		return fmt.Errorf("failed waiting for coordinator to become healthy: %w", err)
	}

	logger.Info(ctx, "Starting polling goroutines",
		"worker_id", w.id,
		"max_concurrent_runs", w.maxConcurrentRuns)

	// Create a wait group to track all polling goroutines
	var wg sync.WaitGroup

	// Launch polling goroutines
	for i := 0; i < w.maxConcurrentRuns; i++ {
		wg.Add(1)
		go func(pollerIndex int) {
			defer wg.Done()
			w.runPoller(ctx, pollerIndex)
		}(i)
	}

	// Wait for all pollers to complete
	wg.Wait()

	return nil
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop(ctx context.Context) error {
	logger.Info(ctx, "Worker stopping", "worker_id", w.id)

	if w.conn != nil {
		if err := w.conn.Close(); err != nil {
			return fmt.Errorf("failed to close gRPC connection: %w", err)
		}
	}

	return nil
}

// waitForHealthy waits for the coordinator to become healthy using exponential backoff with jitter.
func (w *Worker) waitForHealthy(ctx context.Context) error {
	// Create exponential backoff policy with jitter for health checks
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Minute
	basePolicy.MaxRetries = 0 // Retry indefinitely

	// Add full jitter to prevent thundering herd
	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)
	retrier := backoff.NewRetrier(policy)

	logger.Info(ctx, "Waiting for coordinator to become healthy",
		"worker_id", w.id,
		"coordinator", w.coordinatorAddr)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("health check canceled: %w", ctx.Err())
		default:
			// Perform health check
			req := &grpc_health_v1.HealthCheckRequest{
				Service: "", // Check overall server health
			}

			resp, err := w.healthClient.Check(ctx, req)
			if err == nil && resp.Status == grpc_health_v1.HealthCheckResponse_SERVING {
				logger.Info(ctx, "Coordinator is healthy",
					"worker_id", w.id,
					"coordinator", w.coordinatorAddr)
				return nil
			}

			// Log the health check failure
			logger.Warn(ctx, "Health check failed",
				"worker_id", w.id,
				"coordinator", w.coordinatorAddr,
				"err", err)

			// Apply backoff before retrying
			retryErr := retrier.Next(ctx, err)
			if retryErr != nil {
				if retryErr == backoff.ErrOperationCanceled {
					return fmt.Errorf("health check canceled during backoff: %w", ctx.Err())
				}
				// This shouldn't happen with MaxRetries = 0
				return fmt.Errorf("health check retry exhausted: %w", retryErr)
			}
		}
	}
}

// runPoller runs a single polling loop that continuously polls for tasks.
func (w *Worker) runPoller(ctx context.Context, pollerIndex int) {
	// Create exponential backoff policy with jitter
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Minute
	basePolicy.MaxRetries = 0 // Retry indefinitely

	// Add full jitter to prevent thundering herd when multiple pollers reconnect
	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)
	retrier := backoff.NewRetrier(policy)

	for {
		select {
		case <-ctx.Done():
			logger.Debug(ctx, "Poller stopping due to context cancellation",
				"worker_id", w.id,
				"poller_index", pollerIndex)
			return

		default:
			// Generate a fresh UUID for this poll request
			pollerID := uuid.New().String()

			// Create poll request
			req := &coordinatorv1.PollRequest{
				WorkerId: w.id,
				PollerId: pollerID,
			}

			// Perform the poll (this is a long-polling call)
			resp, err := w.client.Poll(ctx, req)
			if err != nil {
				logger.Error(ctx, "Poll failed",
					"error", err,
					"worker_id", w.id,
					"poller_id", pollerID)

				// Check if this is a connection error that requires health checking
				if isConnectionError(err) {
					logger.Warn(ctx, "Connection error detected, switching to health check mode",
						"worker_id", w.id,
						"poller_index", pollerIndex,
						"error", err)

					// Wait for coordinator to become healthy again
					if healthErr := w.waitForHealthy(ctx); healthErr != nil {
						logger.Error(ctx, "Failed during health check recovery",
							"worker_id", w.id,
							"poller_index", pollerIndex,
							"error", healthErr)
						return
					}

					// Health check succeeded, reset retrier and continue polling
					retrier = backoff.NewRetrier(policy)
					logger.Info(ctx, "Resuming polling after health check recovery",
						"worker_id", w.id,
						"poller_index", pollerIndex)
					continue
				}

				// For non-connection errors, apply normal backoff
				retryErr := retrier.Next(ctx, err)
				if retryErr != nil {
					if retryErr == backoff.ErrOperationCanceled {
						logger.Debug(ctx, "Poller retry canceled",
							"worker_id", w.id,
							"poller_index", pollerIndex)
						return
					}
					// This shouldn't happen with MaxRetries = 0, but log it just in case
					logger.Error(ctx, "Retry exhausted",
						"error", retryErr,
						"worker_id", w.id,
						"poller_index", pollerIndex)
					return
				}
				continue
			}

			// Reset retrier on successful poll
			retrier = backoff.NewRetrier(policy)

			// Handle the received task
			if resp.Task != nil {
				logger.Info(ctx, "Task received",
					"worker_id", w.id,
					"poller_id", pollerID,
					"root_dag_run_name", resp.Task.RootDagRunName,
					"root_dag_run_id", resp.Task.RootDagRunId,
					"parent_dag_run_name", resp.Task.ParentDagRunName,
					"parent_dag_run_id", resp.Task.ParentDagRunId,
					"dag_run_id", resp.Task.DagRunId)

				// TODO: Execute the task
				// For now, we just log that we received it
			}
		}
	}
}

// getDialOptions returns the appropriate gRPC dial options based on TLS configuration
func (w *Worker) getDialOptions() ([]grpc.DialOption, error) {
	if w.tlsConfig == nil || w.tlsConfig.Insecure {
		// Use insecure connection (h2c)
		return []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		}, nil
	}

	// Configure TLS
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	// Set InsecureSkipVerify if requested
	if w.tlsConfig.SkipTLSVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	// Load client certificates if provided
	if w.tlsConfig.CertFile != "" && w.tlsConfig.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(w.tlsConfig.CertFile, w.tlsConfig.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load client certificates: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	// Load CA certificate if provided
	if w.tlsConfig.CAFile != "" {
		caData, err := os.ReadFile(w.tlsConfig.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA certificate: %w", err)
		}

		certPool, err := x509.SystemCertPool()
		if err != nil {
			// Fall back to empty pool
			certPool = x509.NewCertPool()
		}

		if !certPool.AppendCertsFromPEM(caData) {
			return nil, fmt.Errorf("failed to append CA certificate")
		}
		tlsConfig.RootCAs = certPool
	}

	return []grpc.DialOption{
		grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)),
	}, nil
}

// isConnectionError checks if the error indicates a connection problem that requires health checking.
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// Check for gRPC status codes that indicate connection issues
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, could still be a connection issue
		return true
	}

	switch st.Code() {
	case codes.Unavailable, codes.Internal, codes.Unknown:
		// These typically indicate server or connection problems
		return true
	case codes.DeadlineExceeded, codes.Canceled:
		// Could be connection-related timeouts
		return true
	default:
		// Other errors like InvalidArgument, NotFound, etc. are not connection errors
		return false
	}
}
