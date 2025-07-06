package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Worker represents a worker instance that polls for tasks from the coordinator.
type Worker struct {
	id                string
	maxConcurrentRuns int
	coordinatorAddr   string
	client            coordinatorv1.CoordinatorServiceClient
	conn              *grpc.ClientConn
}

// NewWorker creates a new worker instance.
func NewWorker(workerID string, maxConcurrentRuns int, coordinatorHost string, coordinatorPort int) *Worker {
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
	}
}

// Start begins the worker's operation, launching multiple polling goroutines.
func (w *Worker) Start(ctx context.Context) error {
	// Establish gRPC connection
	conn, err := grpc.NewClient(w.coordinatorAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to coordinator at %s: %w", w.coordinatorAddr, err)
	}
	w.conn = conn
	w.client = coordinatorv1.NewCoordinatorServiceClient(conn)

	logger.Info(ctx, "Worker started",
		"worker_id", w.id,
		"coordinator", w.coordinatorAddr,
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

// runPoller runs a single polling loop that continuously polls for tasks.
func (w *Worker) runPoller(ctx context.Context, pollerIndex int) {
	backoff := time.Second
	maxBackoff := time.Minute

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
					"poller_id", pollerID,
					"backoff", backoff)

				// Apply exponential backoff
				time.Sleep(backoff)
				backoff = backoff * 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}

			// Reset backoff on successful poll
			backoff = time.Second

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
