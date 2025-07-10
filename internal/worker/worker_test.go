// Package worker_test provides integration tests for the worker component.
//
// Test Design:
// - These tests use a MockTaskExecutor to control task execution behavior
// - This allows for deterministic testing without relying on real execution timing
// - Tests verify polling, task dispatch, connection handling, and error scenarios
// - The tests will continue to work when actual task execution is implemented
package worker_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dagrun"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/dagu-org/dagu/internal/worker"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockTaskExecutor is a mock implementation of TaskExecutor for testing
type MockTaskExecutor struct {
	ExecuteFunc   func(ctx context.Context, task *coordinatorv1.Task) error
	ExecutedTasks []string
	ExecutionTime time.Duration
	mu            sync.Mutex
}

// Execute implements the TaskExecutor interface
func (m *MockTaskExecutor) Execute(ctx context.Context, task *coordinatorv1.Task) error {
	m.mu.Lock()
	m.ExecutedTasks = append(m.ExecutedTasks, task.DagRunId)
	m.mu.Unlock()

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, task)
	}

	// Default behavior: simulate execution with configurable duration
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

// GetExecutedTasks returns a copy of executed task IDs
func (m *MockTaskExecutor) GetExecutedTasks() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]string, len(m.ExecutedTasks))
	copy(result, m.ExecutedTasks)
	return result
}

// createTestWorker creates a worker with a mock dagrun.Manager for testing
func createTestWorker(workerID string, maxConcurrentRuns int, host string, port int, tlsConfig *worker.TLSConfig) *worker.Worker {
	mockMgr := dagrun.New(nil, nil, "dagu", ".")
	labels := make(map[string]string)
	return worker.NewWorker(workerID, maxConcurrentRuns, host, port, tlsConfig, mockMgr, labels)
}

func TestWorkerConnection(t *testing.T) {
	t.Run("ConnectToCoordinator", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with instant mock executor
		w := createTestWorker("test-worker-1", 1, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker in a goroutine
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Give worker time to start and connect
		time.Sleep(100 * time.Millisecond)

		// Cancel context to stop worker
		cancel()
		wg.Wait()

		// Stop worker
		err := w.Stop(context.Background())
		require.NoError(t, err)
	})

	t.Run("ConnectWithCustomWorkerID", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with custom ID and instant mock executor
		customID := "custom-worker-id"
		w := createTestWorker(customID, 1, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Give worker time to start
		time.Sleep(100 * time.Millisecond)

		// Cancel and wait
		cancel()
		wg.Wait()
	})
}

func TestWorkerPolling(t *testing.T) {
	t.Run("ReceiveTask", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create mock executor
		mockExecutor := &MockTaskExecutor{
			ExecutionTime: 100 * time.Millisecond, // Fast execution for tests
		}

		// Create worker
		w := createTestWorker("test-worker", 1, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(mockExecutor)

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for worker to start polling
		time.Sleep(200 * time.Millisecond)

		// Dispatch a task
		task := &coordinatorv1.Task{
			RootDagRunName:   "test-dag",
			RootDagRunId:     "root-123",
			ParentDagRunName: "parent-dag",
			ParentDagRunId:   "parent-456",
			DagRunId:         "run-789",
		}

		err := coord.DispatchTask(t, task)
		require.NoError(t, err)

		// Wait for task to be dispatched and executed
		time.Sleep(300 * time.Millisecond)

		// Verify the task was executed
		executedTasks := mockExecutor.GetExecutedTasks()
		require.Len(t, executedTasks, 1)
		require.Equal(t, "run-789", executedTasks[0])

		// Cancel and wait
		cancel()
		wg.Wait()
	})

	t.Run("MultipleWorkers", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create shared mock executor to track all executed tasks
		mockExecutor := &MockTaskExecutor{
			ExecutionTime: 50 * time.Millisecond,
		}

		// Create multiple workers
		workers := make([]*worker.Worker, 3)
		for i := 0; i < 3; i++ {
			workers[i] = createTestWorker(
				"",
				2, // 2 concurrent runs each
				"127.0.0.1",
				coord.Port(),
				&worker.TLSConfig{Insecure: true},
			)
			workers[i].SetTaskExecutor(mockExecutor)
		}

		// Start all workers
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		for _, w := range workers {
			wg.Add(1)
			w := w
			go func() {
				defer wg.Done()
				err := w.Start(ctx)
				require.NoError(t, err)
			}()
		}

		// Wait for workers to start polling
		time.Sleep(200 * time.Millisecond)

		// Dispatch multiple tasks
		for i := 0; i < 5; i++ {
			task := &coordinatorv1.Task{
				DagRunId: string(rune('a' + i)),
			}
			err := coord.DispatchTask(t, task)
			require.NoError(t, err)
			time.Sleep(50 * time.Millisecond)
		}

		// Wait for all tasks to be executed
		time.Sleep(500 * time.Millisecond)

		// Verify all tasks were executed
		executedTasks := mockExecutor.GetExecutedTasks()
		require.Len(t, executedTasks, 5)

		// Check that all expected tasks were executed
		executedMap := make(map[string]bool)
		for _, taskID := range executedTasks {
			executedMap[taskID] = true
		}
		for i := 0; i < 5; i++ {
			expectedID := string(rune('a' + i))
			require.True(t, executedMap[expectedID], "Task %s was not executed", expectedID)
		}

		// Cancel and wait
		cancel()
		wg.Wait()
	})
}

func TestWorkerReconnection(t *testing.T) {
	t.Run("ReconnectAfterCoordinatorRestart", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)
		port := coord.Port()

		// Create worker with instant mock executor
		w := createTestWorker("test-worker", 1, "127.0.0.1", port, &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for worker to connect
		time.Sleep(200 * time.Millisecond)

		// Stop coordinator (simulate failure)
		err := coord.Stop()
		require.NoError(t, err)

		// Wait a bit
		time.Sleep(500 * time.Millisecond)

		// Worker should still be running, trying to reconnect
		// (We can't easily restart on same port in test, so just verify worker doesn't crash)

		// Cancel and wait
		cancel()
		wg.Wait()
	})
}

func TestWorkerWithTLS(t *testing.T) {
	t.Run("InsecureConnection", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with insecure connection and instant mock executor
		w := createTestWorker("test-worker", 1, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for connection
		time.Sleep(100 * time.Millisecond)

		// Cancel and wait
		cancel()
		wg.Wait()
	})

	t.Run("NilTLSConfig", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with nil TLS config (should default to insecure) and instant mock executor
		w := createTestWorker("test-worker", 1, "127.0.0.1", coord.Port(), nil)
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for connection
		time.Sleep(100 * time.Millisecond)

		// Cancel and wait
		cancel()
		wg.Wait()
	})
}

func TestWorkerShutdown(t *testing.T) {
	t.Run("GracefulShutdown", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with instant mock executor
		w := createTestWorker("test-worker", 2, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for worker to start
		time.Sleep(100 * time.Millisecond)

		// Cancel context for graceful shutdown
		cancel()
		wg.Wait()

		// Stop should work without error
		err := w.Stop(context.Background())
		require.NoError(t, err)
	})

	t.Run("StopWithoutStart", func(t *testing.T) {
		// Create worker with instant mock executor
		w := createTestWorker("test-worker", 1, "127.0.0.1", 50051, &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Stop should work without error even if not started
		err := w.Stop(context.Background())
		require.NoError(t, err)
	})
}

func TestWorkerConcurrency(t *testing.T) {
	t.Run("MaxConcurrentRuns", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		maxConcurrent := 5

		// Create mock executor that holds tasks to simulate long-running execution
		mockExecutor := &MockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, _ *coordinatorv1.Task) error {
				// Hold the task for a long time to ensure all workers become busy
				select {
				case <-time.After(10 * time.Second):
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		}

		// Create worker with specific max concurrent runs
		w := createTestWorker("test-worker", maxConcurrent, "127.0.0.1", coord.Port(), &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(mockExecutor)

		// Start worker
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := w.Start(ctx)
			require.NoError(t, err)
		}()

		// Wait for worker to start all pollers
		time.Sleep(500 * time.Millisecond)

		// Dispatch exactly maxConcurrent tasks
		var dispatchErrors []error
		for i := 0; i < maxConcurrent; i++ {
			task := &coordinatorv1.Task{
				DagRunId: string(rune('a' + i)),
			}
			err := coord.DispatchTask(t, task)
			dispatchErrors = append(dispatchErrors, err)
		}

		// All dispatches should succeed
		for _, err := range dispatchErrors {
			require.NoError(t, err)
		}

		// Try to dispatch one more task - should fail as all pollers are busy
		extraTask := &coordinatorv1.Task{
			DagRunId: "extra",
		}
		err := coord.DispatchTask(t, extraTask)
		require.Error(t, err)
		require.Equal(t, codes.FailedPrecondition, status.Code(err))

		// Cancel and wait
		cancel()
		wg.Wait()
	})
}

func TestWorkerErrorHandling(t *testing.T) {
	t.Run("InvalidCoordinatorAddress", func(t *testing.T) {
		// Create worker with invalid coordinator address and instant mock executor
		w := createTestWorker("test-worker", 1, "invalid-host", 99999, &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start should fail with connection error
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		err := w.Start(ctx)
		// The worker will keep retrying, so we'll get a context deadline exceeded
		require.Error(t, err)
	})

	t.Run("ContextCancellationDuringHealthCheck", func(t *testing.T) {
		// Create worker pointing to non-existent coordinator with instant mock executor
		w := createTestWorker("test-worker", 1, "127.0.0.1", 65535, &worker.TLSConfig{
			Insecure: true,
		})
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start with very short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err := w.Start(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "context")
	})
}
