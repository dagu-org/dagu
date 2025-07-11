package worker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
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

// dagRunTaskExecutor is the implementation that uses dagrun.Manager to execute tasks
type dagRunTaskExecutor struct {
	manager dagrun.Manager
}

// Execute runs the task using the dagrun.Manager
func (e *dagRunTaskExecutor) Execute(ctx context.Context, task *coordinatorv1.Task) error {
	logger.Info(ctx, "Executing task",
		"operation", task.Operation.String(),
		"target", task.Target,
		"dag_run_id", task.DagRunId,
		"root_dag_run_id", task.RootDagRunId,
		"parent_dag_run_id", task.ParentDagRunId)

	err := e.manager.HandleTask(ctx, task)
	if err != nil {
		logger.Error(ctx, "Task execution failed",
			"dag_run_id", task.DagRunId,
			"error", err)
		return err
	}

	logger.Info(ctx, "Task execution completed successfully",
		"dag_run_id", task.DagRunId)
	return nil
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
	labels            map[string]string

	// For tracking poller states and heartbeats
	pollersMu    sync.Mutex
	runningTasks map[string]*coordinatorv1.RunningTask // pollerID -> running task
}

// SetTaskExecutor sets a custom task executor for testing or custom execution logic
func (w *Worker) SetTaskExecutor(executor TaskExecutor) {
	w.taskExecutor = executor
}

// NewWorker creates a new worker instance.
func NewWorker(workerID string, maxConcurrentRuns int, coordinatorHost string, coordinatorPort int, tlsConfig *TLSConfig, dagRunMgr dagrun.Manager, labels map[string]string) *Worker {
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
		taskExecutor:      &dagRunTaskExecutor{manager: dagRunMgr},
		labels:            labels,
		runningTasks:      make(map[string]*coordinatorv1.RunningTask),
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
			// Create a wrapper task executor that tracks task state
			wrappedExecutor := &trackingTaskExecutor{
				worker:        w,
				pollerIndex:   pollerIndex,
				innerExecutor: w.taskExecutor,
			}
			poller := NewPoller(w.id, w.coordinatorAddr, w.client, wrappedExecutor, pollerIndex, w.labels)
			poller.Run(ctx)
		}(i)
	}

	// Start heartbeat goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.sendHeartbeats(ctx)
	}()

	// Wait for all goroutines to complete
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

// sendHeartbeats sends periodic heartbeats to the coordinator
func (w *Worker) sendHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := w.sendHeartbeat(ctx); err != nil {
				logger.Error(ctx, "Failed to send heartbeat",
					"worker_id", w.id,
					"error", err)
			}
		}
	}
}

// sendHeartbeat sends a single heartbeat to the coordinator
func (w *Worker) sendHeartbeat(ctx context.Context) error {
	w.pollersMu.Lock()

	// Calculate stats
	busyCount := len(w.runningTasks)
	runningTasks := make([]*coordinatorv1.RunningTask, 0, busyCount)
	for _, task := range w.runningTasks {
		runningTasks = append(runningTasks, task)
	}

	w.pollersMu.Unlock()

	// Safely convert to int32, capping at max int32 if needed
	totalPollers := int32(math.MaxInt32)
	if w.maxConcurrentRuns <= math.MaxInt32 {
		totalPollers = int32(w.maxConcurrentRuns) //nolint:gosec // Already checked above
	}

	busyCount32 := int32(math.MaxInt32)
	if busyCount <= math.MaxInt32 {
		busyCount32 = int32(busyCount) //nolint:gosec // Already checked above
	}

	req := &coordinatorv1.HeartbeatRequest{
		WorkerId: w.id,
		Labels:   w.labels,
		Stats: &coordinatorv1.WorkerStats{
			TotalPollers: totalPollers,
			BusyPollers:  busyCount32,
			RunningTasks: runningTasks,
		},
	}

	_, err := w.client.Heartbeat(ctx, req)
	return err
}

// trackingTaskExecutor wraps a TaskExecutor to track running tasks
type trackingTaskExecutor struct {
	worker        *Worker
	pollerIndex   int
	innerExecutor TaskExecutor
}

// Execute tracks task state and delegates to the inner executor
func (t *trackingTaskExecutor) Execute(ctx context.Context, task *coordinatorv1.Task) error {
	pollerID := fmt.Sprintf("poller-%d", t.pollerIndex)

	// Mark task as running
	t.worker.pollersMu.Lock()
	t.worker.runningTasks[pollerID] = &coordinatorv1.RunningTask{
		DagRunId:  task.DagRunId,
		DagName:   task.Target,
		StartedAt: time.Now().Unix(),
	}
	t.worker.pollersMu.Unlock()

	// Execute the task
	err := t.innerExecutor.Execute(ctx, task)

	// Remove from running tasks
	t.worker.pollersMu.Lock()
	delete(t.worker.runningTasks, pollerID)
	t.worker.pollersMu.Unlock()

	return err
}
