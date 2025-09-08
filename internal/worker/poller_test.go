package worker_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/backoff"
	"github.com/dagu-org/dagu/internal/coordinator"
	"github.com/dagu-org/dagu/internal/worker"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// TestPollerStateTracking tests that the poller correctly tracks connection state via coordinator client
func TestPollerStateTracking(t *testing.T) {
	t.Run("InitialStateIsConnected", func(t *testing.T) {
		mockCoordinatorCli := newMockCoordinatorCli()
		mockExecutor := &mockTaskExecutor{}
		labels := make(map[string]string)

		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Check initial state
		isConnected, consecutiveFails, lastError := poller.GetState()
		assert.True(t, isConnected)
		assert.Equal(t, 0, consecutiveFails)
		assert.Nil(t, lastError)
	})

	t.Run("StateReflectsDispatcherMetrics", func(t *testing.T) {
		mockCoordinatorCli := newMockCoordinatorCli()
		connectionError := status.Error(codes.Unavailable, "connection refused")

		// Configure coordinator client to return specific metrics
		mockCoordinatorCli.MetricsFunc = func() coordinator.Metrics {
			return coordinator.Metrics{
				IsConnected:      false,
				ConsecutiveFails: 5,
				LastError:        connectionError,
			}
		}

		mockExecutor := &mockTaskExecutor{}
		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// State should reflect coordinator client metrics
		isConnected, consecutiveFails, lastError := poller.GetState()
		assert.False(t, isConnected)
		assert.Equal(t, 5, consecutiveFails)
		assert.Equal(t, connectionError, lastError)
	})
}

func TestPollerTaskDispatch(t *testing.T) {
	t.Run("DispatchTaskToExecutor", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expectedTask := &coordinatorv1.Task{
			DagRunId: "test-run-123",
			Target:   "test.yaml",
		}

		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(ctx context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			// Return task once then nil
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// Return task on first call, then block
				return expectedTask, nil
			}
		}

		var executedTask *coordinatorv1.Task
		mockExecutor := &mockTaskExecutor{
			ExecuteFunc: func(_ context.Context, task *coordinatorv1.Task) error {
				executedTask = task
				cancel() // Stop poller after executing task
				return nil
			},
		}

		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller
		poller.Run(ctx)

		// Verify task was executed
		require.NotNil(t, executedTask)
		assert.Equal(t, expectedTask.DagRunId, executedTask.DagRunId)
		assert.Equal(t, expectedTask.Target, executedTask.Target)
	})

	t.Run("ContinuePollingAfterTaskExecution", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var pollCount int32
		var executionCount int32

		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(ctx context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			count := atomic.AddInt32(&pollCount, 1)

			// Return tasks for first 3 polls
			if count <= 3 {
				return &coordinatorv1.Task{
					DagRunId: fmt.Sprintf("run-%d", count),
				}, nil
			}

			// Then just wait
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(100 * time.Millisecond):
				return nil, nil
			}
		}

		mockExecutor := &mockTaskExecutor{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				atomic.AddInt32(&executionCount, 1)
				return nil
			},
		}

		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller in background
		go poller.Run(ctx)

		// Wait for executions
		time.Sleep(500 * time.Millisecond)
		cancel()

		// Should have executed 3 tasks
		assert.Equal(t, int32(3), atomic.LoadInt32(&executionCount))
		assert.GreaterOrEqual(t, atomic.LoadInt32(&pollCount), int32(3))
	})
}

func TestPollerErrorHandling(t *testing.T) {
	t.Run("HandleExecutorError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mockCoordinatorCli := newMockCoordinatorCli()
		var taskReturned bool
		mockCoordinatorCli.PollFunc = func(ctx context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			if !taskReturned {
				taskReturned = true
				return &coordinatorv1.Task{DagRunId: "error-task"}, nil
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(50 * time.Millisecond):
				return nil, nil
			}
		}

		executorError := fmt.Errorf("execution failed")
		var executionAttempted atomic.Bool
		mockExecutor := &mockTaskExecutor{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				executionAttempted.Store(true)
				return executorError
			},
		}

		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller in background
		go poller.Run(ctx)

		// Wait for execution attempt
		time.Sleep(200 * time.Millisecond)
		cancel()

		// Verify execution was attempted despite error
		assert.True(t, executionAttempted.Load())
	})

	t.Run("ContinueOnPollError", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		var pollAttempts int32
		pollError := status.Error(codes.Unavailable, "poll failed")

		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(_ context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			count := atomic.AddInt32(&pollAttempts, 1)

			if count <= 3 {
				// Fail first 3 attempts
				return nil, pollError
			}

			// Then return a task
			return &coordinatorv1.Task{DagRunId: "success-after-retry"}, nil
		}

		var taskExecuted atomic.Bool
		mockExecutor := &mockTaskExecutor{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				taskExecuted.Store(true)
				cancel() // Stop after execution
				return nil
			},
		}

		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller (will retry on errors)
		poller.Run(ctx)

		// Should have retried and eventually succeeded
		assert.True(t, taskExecuted.Load())
		assert.GreaterOrEqual(t, atomic.LoadInt32(&pollAttempts), int32(4))
	})
}

func TestPollerContextCancellation(t *testing.T) {
	t.Run("StopOnContextCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var pollStarted atomic.Bool
		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(ctx context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			pollStarted.Store(true)
			// Block until cancelled
			<-ctx.Done()
			return nil, ctx.Err()
		}

		mockExecutor := &mockTaskExecutor{}
		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller in background
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			poller.Run(ctx)
		}()

		// Wait for poll to start
		time.Sleep(100 * time.Millisecond)
		assert.True(t, pollStarted.Load(), "Poll should have started")

		// Cancel and wait for completion
		cancel()
		wg.Wait()
	})

	t.Run("StopExecutionOnContextCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(_ context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			return &coordinatorv1.Task{DagRunId: "long-task"}, nil
		}

		var executionStarted atomic.Bool
		mockExecutor := &mockTaskExecutor{
			ExecuteFunc: func(ctx context.Context, _ *coordinatorv1.Task) error {
				executionStarted.Store(true)
				// Simulate long-running task
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(5 * time.Second):
					return nil
				}
			},
		}

		labels := make(map[string]string)
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, labels)

		// Run poller in background
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			poller.Run(ctx)
		}()

		// Wait for execution to start
		time.Sleep(100 * time.Millisecond)
		assert.True(t, executionStarted.Load())

		// Cancel should stop execution
		cancel()
		wg.Wait()
	})
}

func TestPollerWithLabels(t *testing.T) {
	t.Run("SendLabelsInPollRequest", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		expectedLabels := map[string]string{
			"region": "us-east-1",
			"type":   "gpu",
		}

		var receivedReq *coordinatorv1.PollRequest
		mockCoordinatorCli := newMockCoordinatorCli()
		mockCoordinatorCli.PollFunc = func(_ context.Context, _ backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
			receivedReq = req
			cancel() // Stop after first poll
			return nil, nil
		}

		mockExecutor := &mockTaskExecutor{}
		poller := worker.NewPoller("test-worker", mockCoordinatorCli, mockExecutor, 0, expectedLabels)

		// Run poller
		poller.Run(ctx)

		// Verify labels were sent
		require.NotNil(t, receivedReq)
		assert.Equal(t, "test-worker", receivedReq.WorkerId)
		assert.Equal(t, expectedLabels, receivedReq.Labels)
	})
}

// mockCoordinatorCli is a mock implementation of coordinator.Client
type mockCoordinatorCli struct {
	PollFunc     func(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error)
	DispatchFunc func(ctx context.Context, task *coordinatorv1.Task) error
	MetricsFunc  func() coordinator.Metrics
	CleanupFunc  func(ctx context.Context) error

	// Internal state tracking
	mu               sync.Mutex
	isConnected      bool
	consecutiveFails int
	lastError        error
}

func newMockCoordinatorCli() *mockCoordinatorCli {
	return &mockCoordinatorCli{
		isConnected: true,
	}
}

func (m *mockCoordinatorCli) Poll(ctx context.Context, policy backoff.RetryPolicy, req *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
	if m.PollFunc != nil {
		task, err := m.PollFunc(ctx, policy, req)
		m.updateState(err)
		return task, err
	}
	return nil, nil
}

func (m *mockCoordinatorCli) Dispatch(ctx context.Context, task *coordinatorv1.Task) error {
	if m.DispatchFunc != nil {
		return m.DispatchFunc(ctx, task)
	}
	return nil
}

func (m *mockCoordinatorCli) Metrics() coordinator.Metrics {
	if m.MetricsFunc != nil {
		return m.MetricsFunc()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	return coordinator.Metrics{
		IsConnected:      m.isConnected,
		ConsecutiveFails: m.consecutiveFails,
		LastError:        m.lastError,
	}
}

func (m *mockCoordinatorCli) Cleanup(ctx context.Context) error {
	if m.CleanupFunc != nil {
		return m.CleanupFunc(ctx)
	}
	return nil
}

func (m *mockCoordinatorCli) GetWorkers(_ context.Context) ([]*coordinatorv1.WorkerInfo, error) {
	// Return empty list by default for tests
	return []*coordinatorv1.WorkerInfo{}, nil
}

func (m *mockCoordinatorCli) Heartbeat(_ context.Context, _ *coordinatorv1.HeartbeatRequest) error {
	// Return success by default for tests
	return nil
}

func (m *mockCoordinatorCli) updateState(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err != nil {
		m.isConnected = false
		m.consecutiveFails++
		m.lastError = err
	} else {
		m.isConnected = true
		m.consecutiveFails = 0
		m.lastError = nil
	}
}
