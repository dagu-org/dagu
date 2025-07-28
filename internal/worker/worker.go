package worker

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/coordinator/dispatcher"
	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/logger"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

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

	// Use the HandleTask method which handles all operations
	return e.manager.HandleTask(ctx, task)
}

// Worker represents a worker instance that polls for tasks from the coordinator.
type Worker struct {
	id              string
	maxActiveRuns   int
	dispatcher      dispatcher.Client
	taskExecutor    TaskExecutor
	labels          map[string]string

	// For tracking poller states and heartbeats
	pollersMu    sync.Mutex
	runningTasks map[string]*coordinatorv1.RunningTask // pollerID -> running task
}

// SetTaskExecutor sets a custom task executor for testing or custom execution logic
func (w *Worker) SetTaskExecutor(executor TaskExecutor) {
	w.taskExecutor = executor
}

// NewWorker creates a new worker instance.
func NewWorker(workerID string, maxActiveRuns int, dispatcherClient dispatcher.Client, dagRunMgr dagrun.Manager, labels map[string]string) *Worker {
	// Generate default worker ID if not provided
	if workerID == "" {
		hostname, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		}
		workerID = fmt.Sprintf("%s@%d", hostname, os.Getpid())
	}

	return &Worker{
		id:              workerID,
		maxActiveRuns:   maxActiveRuns,
		dispatcher:      dispatcherClient,
		taskExecutor:    &dagRunTaskExecutor{manager: dagRunMgr},
		labels:          labels,
		runningTasks:    make(map[string]*coordinatorv1.RunningTask),
	}
}

// Start begins the worker's operation, launching multiple polling goroutines.
func (w *Worker) Start(ctx context.Context) error {
	logger.Info(ctx, "Starting worker",
		"worker_id", w.id,
		"max_active_runs", w.maxActiveRuns)

	// Create a wait group to track all polling goroutines
	var wg sync.WaitGroup

	// Launch polling goroutines
	for i := 0; i < w.maxActiveRuns; i++ {
		wg.Add(1)
		go func(pollerIndex int) {
			defer wg.Done()
			// Create a wrapper task executor that tracks task state
			wrappedExecutor := &trackingTaskExecutor{
				worker:        w,
				pollerIndex:   pollerIndex,
				innerExecutor: w.taskExecutor,
			}
			poller := NewPoller(w.id, w.dispatcher, wrappedExecutor, pollerIndex, w.labels)
			poller.Run(ctx)
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()

	return nil
}

// Stop gracefully shuts down the worker.
func (w *Worker) Stop(ctx context.Context) error {
	logger.Info(ctx, "Worker stopping", "worker_id", w.id)

	// Cleanup dispatcher connections
	if err := w.dispatcher.Cleanup(ctx); err != nil {
		return fmt.Errorf("failed to cleanup dispatcher: %w", err)
	}

	return nil
}

// trackingTaskExecutor wraps a TaskExecutor to track running task state
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
	err := t.innerExecutor.Execute(ctx, task)

	// Remove from running tasks
	t.worker.pollersMu.Lock()
	delete(t.worker.runningTasks, pollerID)
	t.worker.pollersMu.Unlock()

	return err
}