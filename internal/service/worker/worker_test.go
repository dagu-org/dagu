package worker_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/service/worker"
	"github.com/dagu-org/dagu/internal/test"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkerStart(t *testing.T) {
	t.Run("StartAndStop", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create worker
		w := createTestWorker(t, "test-worker", 5, coord)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker in background
		done := make(chan error, 1)
		go func() {
			done <- w.Start(ctx)
		}()

		// Give worker time to start polling
		time.Sleep(100 * time.Millisecond)

		// Stop worker by cancelling context
		cancel()

		// Should exit cleanly
		select {
		case err := <-done:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Worker did not stop within timeout")
		}

		// Cleanup should succeed
		err := w.Stop(context.Background())
		assert.NoError(t, err)

		// Cleanup
		th.Cleanup()
	})

	t.Run("MultiplePollers", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		maxActiveRuns := 3
		w := createTestWorker(t, "test-worker", maxActiveRuns, coord)

		ctx, cancel := context.WithCancel(context.Background())

		// Track polling activity
		var pollCount atomic.Int32
		w.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				pollCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				return nil
			},
		})

		// Start worker first
		go func() {
			_ = w.Start(ctx)
		}()

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Dispatch multiple tasks
		// Note: We have 3 pollers, so we can dispatch 3 tasks immediately
		// For the remaining tasks, we need to allow time for workers to re-poll
		for i := 0; i < 5; i++ {
			task := &coordinatorv1.Task{
				DagRunId:  "run-" + string(rune('a'+i)),
				Target:    "test.yaml",
				Operation: coordinatorv1.Operation_OPERATION_START,
			}
			err := coord.DispatchTask(t, task)
			require.NoError(t, err)

			// After dispatching first 3 tasks, add delay to allow workers to complete
			// and re-poll (tasks take 100ms to execute in this test)
			if i >= 2 {
				time.Sleep(120 * time.Millisecond)
			}
		}

		// Wait for tasks to be processed
		time.Sleep(200 * time.Millisecond)

		// Should have processed multiple tasks
		assert.GreaterOrEqual(t, pollCount.Load(), int32(3))

		// Stop worker
		cancel()

		// Cleanup
		err := w.Stop(context.Background())
		assert.NoError(t, err)
		th.Cleanup()
	})
}

func TestWorkerTaskExecution(t *testing.T) {
	t.Run("ExecuteDispatchedTask", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create task
		expectedTask := &coordinatorv1.Task{
			DagRunId:  "test-run-123",
			Target:    "test.yaml",
			Operation: coordinatorv1.Operation_OPERATION_START,
		}

		// Create worker
		w := createTestWorker(t, "test-worker", 1, coord)

		// Track task execution
		var executedTask *coordinatorv1.Task
		var wg sync.WaitGroup
		wg.Add(1)

		w.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, task *coordinatorv1.Task) error {
				executedTask = task
				wg.Done()
				return nil
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker first
		go func() {
			_ = w.Start(ctx)
		}()

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Dispatch task to coordinator
		err := coord.DispatchTask(t, expectedTask)
		require.NoError(t, err)

		// Wait for task execution
		done := make(chan bool)
		go func() {
			wg.Wait()
			done <- true
		}()

		select {
		case <-done:
			// Task executed
		case <-time.After(5 * time.Second):
			t.Fatal("Task was not executed within timeout")
		}

		// Verify task was executed
		require.NotNil(t, executedTask)
		assert.Equal(t, expectedTask.DagRunId, executedTask.DagRunId)
		assert.Equal(t, expectedTask.Target, executedTask.Target)

		// Stop worker
		cancel()
		_ = w.Stop(context.Background())

		// Cleanup
		th.Cleanup()
	})

	t.Run("HandleTaskExecutionError", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create task
		task := &coordinatorv1.Task{
			DagRunId:  "error-run-123",
			Target:    "test.yaml",
			Operation: coordinatorv1.Operation_OPERATION_START,
		}

		// Create worker with failing executor
		w := createTestWorker(t, "test-worker", 1, coord)

		var executionAttempted atomic.Bool
		w.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				executionAttempted.Store(true)
				return assert.AnError
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker first
		go func() {
			_ = w.Start(ctx)
		}()

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Dispatch task
		err := coord.DispatchTask(t, task)
		require.NoError(t, err)

		// Wait for execution attempt
		time.Sleep(200 * time.Millisecond)

		// Task should have been attempted despite error
		assert.True(t, executionAttempted.Load())

		// Stop worker
		cancel()
		_ = w.Stop(context.Background())

		// Cleanup
		th.Cleanup()
	})
}

func TestWorkerWithLabels(t *testing.T) {
	t.Run("WorkerWithSelectorLabels", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create task that requires specific labels
		task := &coordinatorv1.Task{
			DagRunId:       "labeled-run-123",
			Target:         "test.yaml",
			Operation:      coordinatorv1.Operation_OPERATION_START,
			WorkerSelector: map[string]string{"type": "special", "region": "us-east"},
		}

		// Create worker WITHOUT matching labels
		w1 := createTestWorker(t, "worker-1", 1, coord)
		var w1Executed atomic.Bool
		w1.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				w1Executed.Store(true)
				return nil
			},
		})

		// Create worker WITH matching labels
		w2 := worker.NewWorker(
			"worker-2",
			1,
			coord.GetCoordinatorClient(t),
			map[string]string{"type": "special", "region": "us-east", "extra": "value"},
			th.Config,
		)
		var w2Executed atomic.Bool
		w2.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				w2Executed.Store(true)
				return nil
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start both workers
		go func() { _ = w1.Start(ctx) }()
		go func() { _ = w2.Start(ctx) }()

		// Give workers time to connect
		time.Sleep(100 * time.Millisecond)

		// Dispatch task
		err := coord.DispatchTask(t, task)
		require.NoError(t, err)

		// Wait for task execution
		time.Sleep(400 * time.Millisecond)

		// Only worker with matching labels should execute
		assert.False(t, w1Executed.Load(), "Worker without labels should not execute")
		assert.True(t, w2Executed.Load(), "Worker with labels should execute")

		// Stop workers
		cancel()
		_ = w1.Stop(context.Background())
		_ = w2.Stop(context.Background())

		// Cleanup
		th.Cleanup()
	})
}

func TestWorkerHeartbeat(t *testing.T) {
	t.Run("SendsHeartbeats", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create worker
		w := createTestWorker(t, "heartbeat-worker", 3, coord)

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Start worker
		go func() {
			_ = w.Start(ctx)
		}()

		// Wait for heartbeats to be sent
		time.Sleep(2 * time.Second)

		// Get workers from coordinator to verify heartbeats
		workers, err := coord.GetCoordinatorClient(t).GetWorkers(context.Background())
		require.NoError(t, err)

		// Should have our worker registered
		var found bool
		for _, worker := range workers {
			if worker.WorkerId == "heartbeat-worker" {
				found = true
				assert.Equal(t, int32(3), worker.TotalPollers)
				assert.NotZero(t, worker.LastHeartbeatAt)
				break
			}
		}
		assert.True(t, found, "Worker should be registered via heartbeats")

		// Stop worker
		cancel()
		_ = w.Stop(context.Background())

		// Cleanup
		th.Cleanup()
	})
}

func TestWorkerStopWithoutStart(t *testing.T) {
	t.Run("StopUnstartedWorker", func(t *testing.T) {
		labels := make(map[string]string)

		// Create a mock coordinator client that doesn't connect
		mockCoordinatorCli := newMockCoordinatorCli()
		w := worker.NewWorker("test-worker", 1, mockCoordinatorCli, labels, &config.Config{})
		w.SetHandler(&mockHandler{ExecutionTime: 0})

		// Stop should work without error even if not started
		err := w.Stop(context.Background())
		require.NoError(t, err)
	})
}

func TestRunningTaskTracking(t *testing.T) {
	t.Run("TrackRunningTasks", func(t *testing.T) {
		// Setup test environment
		coord := test.SetupCoordinator(t)
		th := test.Setup(t)

		// Create worker that holds tasks
		w := createTestWorker(t, "task-tracker", 3, coord)

		var activeTasksMu sync.Mutex
		activeTasks := make(map[string]bool)
		taskStarted := make(chan string, 5)

		w.SetHandler(&mockHandler{
			ExecuteFunc: func(_ context.Context, task *coordinatorv1.Task) error {
				activeTasksMu.Lock()
				activeTasks[task.DagRunId] = true
				activeTasksMu.Unlock()

				taskStarted <- task.DagRunId

				// Hold task for a bit
				time.Sleep(100 * time.Millisecond)

				activeTasksMu.Lock()
				delete(activeTasks, task.DagRunId)
				activeTasksMu.Unlock()
				return nil
			},
		})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start worker
		go func() {
			_ = w.Start(ctx)
		}()

		// Give worker time to connect
		time.Sleep(100 * time.Millisecond)

		// Create first 3 tasks to fill all pollers
		for i := 0; i < 3; i++ {
			task := &coordinatorv1.Task{
				DagRunId:       "task-" + string(rune('a'+i)),
				Target:         "test.yaml",
				RootDagRunName: "root-dag",
				RootDagRunId:   "root-123",
			}
			err := coord.DispatchTask(t, task)
			require.NoError(t, err)
		}

		// Wait for all 3 tasks to start
		for i := 0; i < 3; i++ {
			select {
			case <-taskStarted:
				// Task started
			case <-time.After(2 * time.Second):
				t.Fatal("Tasks did not start within timeout")
			}
		}

		// Check that we have 3 active tasks (they should still be running)
		activeTasksMu.Lock()
		activeCount := len(activeTasks)
		activeTasksMu.Unlock()
		assert.Equal(t, 3, activeCount, "Should have 3 active tasks")

		// Now dispatch 2 more tasks after some have completed
		time.Sleep(120 * time.Millisecond) // Wait for at least one task to complete

		for i := 3; i < 5; i++ {
			task := &coordinatorv1.Task{
				DagRunId:       "task-" + string(rune('a'+i)),
				Target:         "test.yaml",
				RootDagRunName: "root-dag",
				RootDagRunId:   "root-123",
			}
			err := coord.DispatchTask(t, task)
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond) // Small delay between tasks
		}

		// Stop worker
		cancel()
		_ = w.Stop(context.Background())

		// Cleanup
		th.Cleanup()
	})
}

func TestWorkerConnectionFailure(t *testing.T) {
	t.Run("HandleConnectionFailure", func(t *testing.T) {
		// Create worker with a mock coordinator client that always fails
		var pollCount atomic.Int32
		connectionError := status.Error(codes.Unavailable, "connection failed")

		mockCoordinatorCli := newMockCoordinatorCli()

		mockCoordinatorCli.PollFunc = func(_ context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			pollCount.Add(1)
			return nil, connectionError
		}

		labels := make(map[string]string)
		w := worker.NewWorker("test-worker", 1, mockCoordinatorCli, labels, &config.Config{})
		w.SetHandler(&mockHandler{})

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Start worker
		go func() {
			_ = w.Start(ctx)
		}()

		// Wait for context timeout
		<-ctx.Done()

		// Should have attempted multiple polls despite failures
		assert.Greater(t, pollCount.Load(), int32(1), "Should retry on connection failures")

		// Stop worker
		_ = w.Stop(context.Background())

		// Verify coordinator client is in failed state
		metrics := mockCoordinatorCli.Metrics()
		assert.False(t, metrics.IsConnected)
		assert.Greater(t, metrics.ConsecutiveFails, 0)
		assert.NotNil(t, metrics.LastError)
	})
}

type mockHandler struct {
	ExecuteFunc   func(context.Context, *coordinatorv1.Task) error
	ExecutionTime time.Duration
}

func (m *mockHandler) Handle(ctx context.Context, task *coordinatorv1.Task) error {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, task)
	}

	// Default behavior: simulate task execution
	if m.ExecutionTime > 0 {
		select {
		case <-time.After(m.ExecutionTime):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return nil
}

// createTestWorker creates a worker with a mock dagrun.Manager and coordinator client for testing
func createTestWorker(t *testing.T, workerID string, maxActiveRuns int, coord *test.Coordinator) *worker.Worker {

	// Create coordinator client for the coordinator
	coordinatorClient := coord.GetCoordinatorClient(t)

	labels := make(map[string]string)
	return worker.NewWorker(workerID, maxActiveRuns, coordinatorClient, labels, &config.Config{})
}
