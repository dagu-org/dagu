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
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

// TLSConfig holds TLS configuration for the worker
type TLSConfig struct {
	Insecure      bool
	CertFile      string
	KeyFile       string
	CAFile        string
	SkipTLSVerify bool
}

// TaskExecutor defines the interface for executing tasks
type TaskExecutor interface {
	Execute(ctx context.Context, task *coordinatorv1.Task) error
}

// DefaultTaskExecutor is the default implementation that simulates task execution
type DefaultTaskExecutor struct{}

// Execute simulates task execution with a short delay
func (e *DefaultTaskExecutor) Execute(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing task (TODO: implement actual execution)",
		"dag_run_id", task.DagRunId)

	// Simulate task execution time (100ms for testing)
	select {
	case <-time.After(100 * time.Millisecond):
		logger.Info(ctx, "Task execution completed (simulated)",
			"dag_run_id", task.DagRunId)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
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
	taskExecutor      TaskExecutor
}

// SetTaskExecutor sets a custom task executor for testing or custom execution logic
func (w *Worker) SetTaskExecutor(executor TaskExecutor) {
	w.taskExecutor = executor
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
		taskExecutor:      &DefaultTaskExecutor{},
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
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Minute
	basePolicy.MaxRetries = 0 // Retry indefinitely

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	logger.Info(ctx, "Waiting for coordinator to become healthy",
		"worker_id", w.id,
		"coordinator", w.coordinatorAddr)

	return backoff.Retry(ctx, func(ctx context.Context) error {
		req := &grpc_health_v1.HealthCheckRequest{
			Service: "", // Check overall server health
		}

		resp, err := w.healthClient.Check(ctx, req)
		if err != nil {
			logger.Warn(ctx, "Health check failed",
				"worker_id", w.id,
				"coordinator", w.coordinatorAddr,
				"err", err)
			return err
		}

		if resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			err := fmt.Errorf("coordinator not healthy: %s", resp.Status)
			logger.Warn(ctx, "Coordinator not healthy",
				"worker_id", w.id,
				"coordinator", w.coordinatorAddr,
				"status", resp.Status.String())
			return err
		}

		logger.Info(ctx, "Coordinator is healthy",
			"worker_id", w.id,
			"coordinator", w.coordinatorAddr)
		return nil
	}, policy, nil)
}

// runPoller runs a single polling loop that continuously polls for tasks.
func (w *Worker) runPoller(ctx context.Context, pollerIndex int) {
	// Set up retry policy for poll failures only
	basePolicy := backoff.NewExponentialBackoffPolicy(time.Second)
	basePolicy.BackoffFactor = 2.0
	basePolicy.MaxInterval = time.Minute
	basePolicy.MaxRetries = 0 // Retry indefinitely

	policy := backoff.WithJitter(basePolicy, backoff.FullJitter)

	for {
		select {
		case <-ctx.Done():
			logger.Debug(ctx, "Poller stopping due to context cancellation",
				"worker_id", w.id,
				"poller_index", pollerIndex)
			return
		default:
			// Poll for a task
			task, err := w.pollForTask(ctx, pollerIndex, policy)
			if err != nil {
				// Context canceled, exit gracefully
				if ctx.Err() != nil {
					return
				}
				// Poll failed, but will be retried by pollForTask
				continue
			}

			// If we got a task, execute it
			if task != nil {
				logger.Info(ctx, "Task received, starting execution",
					"worker_id", w.id,
					"poller_index", pollerIndex,
					"dag_run_id", task.DagRunId)

				// Execute the task using the TaskExecutor
				err := w.taskExecutor.Execute(ctx, task)
				if err != nil {
					if ctx.Err() != nil {
						// Context cancelled, exit gracefully
						return
					}
					logger.Error(ctx, "Task execution failed",
						"worker_id", w.id,
						"poller_index", pollerIndex,
						"dag_run_id", task.DagRunId,
						"error", err)
					// TODO: Report failure back to coordinator
				} else {
					logger.Info(ctx, "Task execution completed successfully",
						"worker_id", w.id,
						"poller_index", pollerIndex,
						"dag_run_id", task.DagRunId)
					// TODO: Report success back to coordinator
				}
			}
			// Continue polling for the next task
		}
	}
}

// pollForTask polls the coordinator for a task with retry on failure
func (w *Worker) pollForTask(ctx context.Context, pollerIndex int, policy backoff.RetryPolicy) (*coordinatorv1.Task, error) {
	pollerID := uuid.New().String()

	var task *coordinatorv1.Task
	err := backoff.Retry(ctx, func(ctx context.Context) error {
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
				"poller_id", pollerID,
				"poller_index", pollerIndex)
			return err // Will be retried with backoff
		}

		// Handle the received task
		if resp.Task != nil {
			logger.Info(ctx, "Task received",
				"worker_id", w.id,
				"poller_id", pollerID,
				"poller_index", pollerIndex,
				"root_dag_run_name", resp.Task.RootDagRunName,
				"root_dag_run_id", resp.Task.RootDagRunId,
				"parent_dag_run_name", resp.Task.ParentDagRunName,
				"parent_dag_run_id", resp.Task.ParentDagRunId,
				"dag_run_id", resp.Task.DagRunId)
			task = resp.Task
		}

		return nil
	}, policy, nil)

	return task, err
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
