package worker

import (
	"context"
	"fmt"
	"math"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// Worker represents a worker instance that polls for tasks from the coordinator.
type Worker struct {
	id             string
	maxActiveRuns  int
	coordinatorCli coordinator.Client
	handler        TaskHandler
	labels         map[string]string

	// For tracking poller states and heartbeats
	pollersMu    sync.Mutex
	runningTasks map[string]*coordinatorv1.RunningTask // pollerID -> running task
}

// SetHandler sets a custom task executor for testing or custom execution logic
func (w *Worker) SetHandler(executor TaskHandler) {
	w.handler = executor
}

// NewWorker creates a new worker instance.
func NewWorker(workerID string, maxActiveRuns int, coordinatorClient coordinator.Client, labels map[string]string, cfg *config.Config) *Worker {
	// Generate default worker ID if not provided
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		workerID = fmt.Sprintf("%s@%d", hostname, os.Getpid())
	}

	return &Worker{
		id:             workerID,
		maxActiveRuns:  maxActiveRuns,
		coordinatorCli: coordinatorClient,
		handler:        &taskHandler{subCmdBuilder: runtime.NewSubCmdBuilder(cfg)},
		labels:         labels,
		runningTasks:   make(map[string]*coordinatorv1.RunningTask),
	}
}

// Start begins the worker's operation, launching multiple polling goroutines.
func (w *Worker) Start(ctx context.Context) error {
	logger.Info(ctx, "Starting worker",
		tag.WorkerID(w.id),
		tag.MaxConcurrency(w.maxActiveRuns))

	// Create a wait group to track all polling goroutines
	var wg sync.WaitGroup

	// Launch polling goroutines
	for i := 0; i < w.maxActiveRuns; i++ {
		wg.Add(1)
		go func(pollerIndex int) {
			defer wg.Done()
			// Create a wrapper task handler that tracks task state
			wrappedHandler := &trackingHandler{
				worker:      w,
				pollerIndex: pollerIndex,
				handler:     w.handler,
			}
			poller := NewPoller(w.id, w.coordinatorCli, wrappedHandler, pollerIndex, w.labels)
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
	logger.Info(ctx, "Worker stopping", tag.WorkerID(w.id))

	// Cleanup coordinator client connections
	if err := w.coordinatorCli.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup coordinator client: %w", err)
	}

	return nil
}

// trackingHandler wraps a TaskHandler to track running task state
type trackingHandler struct {
	worker      *Worker
	pollerIndex int
	handler     TaskHandler
}

// Handle tracks task state and delegates to the inner executor
func (t *trackingHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	pollerID := fmt.Sprintf("poller-%d", t.pollerIndex)

	// Mark task as running
	t.worker.pollersMu.Lock()
	t.worker.runningTasks[pollerID] = &coordinatorv1.RunningTask{
		DagRunId:         task.DagRunId,
		DagName:          task.Target,
		StartedAt:        time.Now().Unix(),
		RootDagRunName:   task.RootDagRunName,
		RootDagRunId:     task.RootDagRunId,
		ParentDagRunName: task.ParentDagRunName,
		ParentDagRunId:   task.ParentDagRunId,
	}
	t.worker.pollersMu.Unlock()

	// Execute the task
	err := t.handler.Handle(ctx, task)

	// Remove from running tasks
	t.worker.pollersMu.Lock()
	delete(t.worker.runningTasks, pollerID)
	t.worker.pollersMu.Unlock()

	return err
}

// sendHeartbeats sends periodic heartbeats to the coordinator
func (w *Worker) sendHeartbeats(ctx context.Context) {
	basePolicy := backoff.NewExponentialBackoffPolicy(1 * time.Second)
	basePolicy.BackoffFactor = 1.5
	basePolicy.MaxInterval = 15 * time.Second
	basePolicy.MaxRetries = 0 // unlimited retries
	retryPolicy := backoff.WithJitter(basePolicy, backoff.Jitter)
	retrier := backoff.NewRetrier(retryPolicy)

	const healthyInterval = 1 * time.Second

	waitWithContext := func(ctx context.Context, d time.Duration) bool {
		if d <= 0 {
			return ctx.Err() == nil
		}
		timer := time.NewTimer(d)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return false
		case <-timer.C:
			return true
		}
	}

	nextDelay := time.Duration(0)

	for {
		if !waitWithContext(ctx, nextDelay) {
			return
		}

		if ctx.Err() != nil {
			return
		}

		if err := w.sendHeartbeat(ctx); err != nil {
			nextInterval, nextErr := retrier.Next(err)
			if nextErr != nil {
				logger.Error(ctx, "Failed to compute heartbeat backoff interval",
				tag.WorkerID(w.id),
				tag.Error(err))
				nextInterval = healthyInterval
			} else {
				logger.Warn(ctx, "Heartbeat send failed; will retry with backoff",
				tag.WorkerID(w.id),
				tag.Error(err),
				tag.Interval(nextInterval))
			}

			nextDelay = nextInterval
			continue
		}

		// Successful heartbeat; reset backoff and schedule healthy interval
		retrier.Reset()
		nextDelay = healthyInterval
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
	if w.maxActiveRuns <= math.MaxInt32 {
		totalPollers = int32(w.maxActiveRuns) //nolint:gosec // Already checked above
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

	return w.coordinatorCli.Heartbeat(ctx, req)
}
