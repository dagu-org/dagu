package worker

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/runtime"
	"github.com/dagu-org/dagu/internal/runtime/builtin/sql"
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
	cfg            *config.Config

	// For tracking poller states and heartbeats
	pollersMu    sync.Mutex
	runningTasks map[string]*coordinatorv1.RunningTask // pollerID -> running task

	// For cancellation support (key is AttemptKey)
	cancelFuncs map[string]context.CancelFunc

	// For graceful shutdown
	stopOnce   sync.Once
	stopCancel context.CancelFunc // Cancels the worker's internal context
	stopDone   chan struct{}      // Signals when all goroutines have stopped

	// For global PostgreSQL connection pool (shared-nothing mode)
	poolManager *sql.GlobalPoolManager
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
		cfg:            cfg,
		runningTasks:   make(map[string]*coordinatorv1.RunningTask),
		cancelFuncs:    make(map[string]context.CancelFunc),
	}
}

// Start begins the worker's operation, launching multiple polling goroutines.
func (w *Worker) Start(ctx context.Context) error {
	logger.Info(ctx, "Starting worker",
		tag.WorkerID(w.id),
		tag.MaxConcurrency(w.maxActiveRuns))

	// Create an internal context that can be cancelled by Stop()
	// This context is cancelled when either the parent context is done OR Stop() is called
	internalCtx, cancel := context.WithCancel(ctx)
	w.stopCancel = cancel
	w.stopDone = make(chan struct{})

	// Initialize global PostgreSQL pool manager if in shared-nothing mode
	if w.isSharedNothingMode() {
		w.poolManager = sql.NewGlobalPoolManager(sql.GlobalPoolConfig{
			MaxOpenConns:    w.cfg.Worker.PostgresPool.MaxOpenConns,
			MaxIdleConns:    w.cfg.Worker.PostgresPool.MaxIdleConns,
			ConnMaxLifetime: time.Duration(w.cfg.Worker.PostgresPool.ConnMaxLifetime) * time.Second,
			ConnMaxIdleTime: time.Duration(w.cfg.Worker.PostgresPool.ConnMaxIdleTime) * time.Second,
		})
		internalCtx = sql.WithPoolManager(internalCtx, w.poolManager)
		logger.Info(ctx, "Global PostgreSQL pool manager initialized",
			tag.WorkerID(w.id),
			slog.Int("maxOpenConns", w.cfg.Worker.PostgresPool.MaxOpenConns),
			slog.Int("maxIdleConns", w.cfg.Worker.PostgresPool.MaxIdleConns))
	}

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
			poller.Run(internalCtx)
		}(i)
	}

	// Start heartbeat goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		w.sendHeartbeats(internalCtx)
	}()

	// Wait for all goroutines to complete, then signal done
	go func() {
		wg.Wait()
		close(w.stopDone)
	}()

	// Block until all goroutines complete
	<-w.stopDone

	return nil
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop(ctx context.Context) error {
	var err error
	w.stopOnce.Do(func() {
		logger.Info(ctx, "Worker stopping", tag.WorkerID(w.id))

		// Cancel the internal context to signal all goroutines to stop
		if w.stopCancel != nil {
			w.stopCancel()
		}

		// Wait for all goroutines to complete (with timeout from ctx)
		if w.stopDone != nil {
			select {
			case <-w.stopDone:
				// All goroutines have stopped
			case <-ctx.Done():
				logger.Warn(ctx, "Worker stop timed out waiting for goroutines",
					tag.WorkerID(w.id))
			}
		}

		// Close the global PostgreSQL pool manager if initialized
		if w.poolManager != nil {
			if poolErr := w.poolManager.Close(); poolErr != nil {
				logger.Error(ctx, "Failed to close PostgreSQL pool manager",
					tag.WorkerID(w.id),
					tag.Error(poolErr))
				if err == nil {
					err = fmt.Errorf("failed to close PostgreSQL pool manager: %w", poolErr)
				}
			} else {
				logger.Info(ctx, "PostgreSQL pool manager closed", tag.WorkerID(w.id))
			}
		}

		// Cleanup coordinator client connections
		if cleanupErr := w.coordinatorCli.Cleanup(ctx); cleanupErr != nil {
			if err == nil {
				err = fmt.Errorf("failed to cleanup coordinator client: %w", cleanupErr)
			}
		}
	})

	return err
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

	// Create a cancellable context for this task
	taskCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Mark task as running and register cancel function
	t.worker.pollersMu.Lock()
	t.worker.runningTasks[pollerID] = &coordinatorv1.RunningTask{
		DagRunId:         task.DagRunId,
		DagName:          task.Target,
		StartedAt:        time.Now().Unix(),
		RootDagRunName:   task.RootDagRunName,
		RootDagRunId:     task.RootDagRunId,
		ParentDagRunName: task.ParentDagRunName,
		ParentDagRunId:   task.ParentDagRunId,
		AttemptKey:       task.AttemptKey,
	}
	t.worker.cancelFuncs[task.AttemptKey] = cancel
	t.worker.pollersMu.Unlock()

	// Execute the task with cancellable context
	err := t.handler.Handle(taskCtx, task)

	// Remove from running tasks and cancel registry
	t.worker.pollersMu.Lock()
	delete(t.worker.runningTasks, pollerID)
	delete(t.worker.cancelFuncs, task.AttemptKey)
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
	totalPollers := int32(min(w.maxActiveRuns, math.MaxInt32)) //nolint:gosec
	busyCount32 := int32(min(busyCount, math.MaxInt32))        //nolint:gosec

	req := &coordinatorv1.HeartbeatRequest{
		WorkerId: w.id,
		Labels:   w.labels,
		Stats: &coordinatorv1.WorkerStats{
			TotalPollers: totalPollers,
			BusyPollers:  busyCount32,
			RunningTasks: runningTasks,
		},
	}

	resp, err := w.coordinatorCli.Heartbeat(ctx, req)
	if err != nil {
		return err
	}

	// Process cancellation directives from the coordinator
	if resp != nil && len(resp.CancelledRuns) > 0 {
		w.processCancellations(ctx, resp.CancelledRuns)
	}

	return nil
}

// processCancellations cancels tasks that the coordinator has marked for cancellation
func (w *Worker) processCancellations(ctx context.Context, cancelledRuns []*coordinatorv1.CancelledRun) {
	w.pollersMu.Lock()
	defer w.pollersMu.Unlock()

	for _, run := range cancelledRuns {
		if cancelFunc, exists := w.cancelFuncs[run.AttemptKey]; exists {
			logger.Info(ctx, "Cancelling task per coordinator directive",
				tag.WorkerID(w.id),
				tag.AttemptKey(run.AttemptKey))
			cancelFunc()
		}
	}
}

// isSharedNothingMode returns true if the worker is running in shared-nothing mode.
// Shared-nothing mode is detected when static coordinator addresses are configured.
// In shared-nothing mode, global PostgreSQL pool management is automatically enabled.
func (w *Worker) isSharedNothingMode() bool {
	return w.cfg != nil && len(w.cfg.Worker.Coordinators) > 0
}
