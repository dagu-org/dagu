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
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/coordinator/dispatcher"
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

// createTestWorker creates a worker with a mock dagrun.Manager and dispatcher for testing
func createTestWorker(t *testing.T, workerID string, maxActiveRuns int, coord *test.Coordinator) *worker.Worker {
	mockMgr := dagrun.New(nil, nil, "dagu", ".")
	labels := make(map[string]string)

	// Create dispatcher client for the coordinator
	dispatcherClient := coord.GetDispatcherClient(t)

	return worker.NewWorker(workerID, maxActiveRuns, dispatcherClient, mockMgr, labels)
}

func TestWorkerConnection(t *testing.T) {
	t.Run("ConnectToCoordinator", func(t *testing.T) {
		// Setup coordinator
		coord := test.SetupCoordinator(t)

		// Create worker with instant mock executor
		w := createTestWorker(t, "test-worker-1", 1, coord)
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
		w := createTestWorker(t, customID, 1, coord)
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
		w := createTestWorker(t, "test-worker", 1, coord)
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
				t,
				"",
				2, // 2 concurrent runs each
				coord,
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

		// Create worker with instant mock executor
		w := createTestWorker(t, "test-worker", 1, coord)
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
		w := createTestWorker(t, "test-worker", 1, coord)
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
		w := createTestWorker(t, "test-worker", 1, coord)
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
		w := createTestWorker(t, "test-worker", 2, coord)
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
		// Create worker with instant mock executor (no coordinator needed)
		mockMgr := dagrun.New(nil, nil, "dagu", ".")
		labels := make(map[string]string)
		// Create a mock dispatcher that doesn't connect
		mockDispatcher := &mockDispatcher{}
		w := worker.NewWorker("test-worker", 1, mockDispatcher, mockMgr, labels)
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Stop should work without error even if not started
		err := w.Stop(context.Background())
		require.NoError(t, err)
	})
}

func TestWorkerConcurrency(t *testing.T) {
	t.Run("MaxActiveRuns", func(t *testing.T) {
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
		w := createTestWorker(t, "test-worker", maxConcurrent, coord)
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

// mockDispatcher is a mock implementation of dispatcher.Client for testing
type mockDispatcher struct {
	pollError    error
	PollFunc     func(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error)
	DispatchFunc func(ctx context.Context, task *coordinatorv1.Task) error

	// State tracking
	consecutiveFails int
}

func (m *mockDispatcher) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {
	if m.DispatchFunc != nil {
		return m.DispatchFunc(ctx, task)
	}
	return m.pollError
}

func (m *mockDispatcher) Poll(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
	if m.PollFunc != nil {
		return m.PollFunc(ctx, policy, req)
	}
	if m.pollError != nil {
		m.consecutiveFails++
		return nil, m.pollError
	}
	m.consecutiveFails = 0
	return nil, nil
}

func (m *mockDispatcher) Metrics() dispatcher.Metrics {
	return dispatcher.Metrics{
		IsConnected:      m.pollError == nil && m.consecutiveFails == 0,
		ConsecutiveFails: m.consecutiveFails,
		LastError:        m.pollError,
	}
}

func (m *mockDispatcher) Cleanup(_ context.Context) error {
	return nil
}

func (m *mockDispatcher) GetWorkers(_ context.Context) ([]*coordinatorv1.WorkerInfo, error) {
	// Return empty list by default for tests
	return []*coordinatorv1.WorkerInfo{}, nil
}

func TestWorkerErrorHandling(t *testing.T) {
	t.Run("DispatcherFailure", func(t *testing.T) {
		// Create worker with a mock dispatcher that always fails
		mockMgr := dagrun.New(nil, nil, "dagu", ".")
		labels := make(map[string]string)

		var pollCalled atomic.Bool
		mockDispatcher := &mockDispatcher{
			pollError: status.Error(codes.Unavailable, "connection failed"),
		}
		// Override the Poll method to track calls
		mockDispatcher.PollFunc = func(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			pollCalled.Store(true)
			mockDispatcher.consecutiveFails++
			return nil, mockDispatcher.pollError
		}

		w := worker.NewWorker("test-worker", 1, mockDispatcher, mockMgr, labels)
		w.SetTaskExecutor(&MockTaskExecutor{ExecutionTime: 0})

		// Start worker with a short timeout
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()

		// Run worker in background
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = w.Start(ctx) // Will run until context is cancelled
		}()

		// Wait a bit to ensure polling started
		time.Sleep(100 * time.Millisecond)

		// Verify that polling was attempted
		require.True(t, pollCalled.Load(), "Poll should have been called")

		// Cancel and wait
		cancel()
		wg.Wait()

		// Verify dispatcher is in failed state
		metrics := mockDispatcher.Metrics()
		require.False(t, metrics.IsConnected)
		require.Greater(t, metrics.ConsecutiveFails, 0)
		require.NotNil(t, metrics.LastError)
	})
}
