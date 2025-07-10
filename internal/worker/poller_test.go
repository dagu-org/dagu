package worker_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/worker"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MockCoordinatorClient is a mock implementation of CoordinatorServiceClient
type MockCoordinatorClient struct {
	PollFunc     func(ctx context.Context, in *coordinatorv1.PollRequest, opts ...grpc.CallOption) (*coordinatorv1.PollResponse, error)
	DispatchFunc func(ctx context.Context, in *coordinatorv1.DispatchRequest, opts ...grpc.CallOption) (*coordinatorv1.DispatchResponse, error)
}

func (m *MockCoordinatorClient) Poll(ctx context.Context, in *coordinatorv1.PollRequest, opts ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
	if m.PollFunc != nil {
		return m.PollFunc(ctx, in, opts...)
	}
	return &coordinatorv1.PollResponse{}, nil
}

func (m *MockCoordinatorClient) Dispatch(ctx context.Context, in *coordinatorv1.DispatchRequest, opts ...grpc.CallOption) (*coordinatorv1.DispatchResponse, error) {
	if m.DispatchFunc != nil {
		return m.DispatchFunc(ctx, in, opts...)
	}
	return &coordinatorv1.DispatchResponse{}, nil
}

// TestPollerStateTracking tests that the poller correctly tracks connection state
func TestPollerStateTracking(t *testing.T) {
	t.Run("InitialStateIsConnected", func(t *testing.T) {
		mockClient := &MockCoordinatorClient{}
		mockExecutor := &MockTaskExecutor{}

		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Check initial state
		isConnected, consecutiveFails, lastError := poller.GetState()
		assert.True(t, isConnected)
		assert.Equal(t, 0, consecutiveFails)
		assert.Nil(t, lastError)
	})

	t.Run("StateChangesOnConnectionFailure", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pollCount := 0
		connectionError := status.Error(codes.Unavailable, "connection refused")

		mockClient := &MockCoordinatorClient{
			PollFunc: func(_ context.Context, _ *coordinatorv1.PollRequest, _ ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
				pollCount++
				if pollCount <= 3 {
					// Fail first 3 attempts
					return nil, connectionError
				}
				// Success on 4th attempt
				return &coordinatorv1.PollResponse{}, nil
			},
		}

		mockExecutor := &MockTaskExecutor{}
		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run poller in background
		go poller.Run(ctx)

		// Wait for first failure
		time.Sleep(100 * time.Millisecond)

		// State should show disconnected
		isConnected, consecutiveFails, lastError := poller.GetState()
		assert.False(t, isConnected)
		assert.Greater(t, consecutiveFails, 0)
		assert.Equal(t, connectionError, lastError)

		// Wait for reconnection (need to wait for exponential backoff)
		// 1s + 2s + 4s = 7s total before 4th attempt succeeds
		maxWait := time.Now().Add(10 * time.Second)
		for time.Now().Before(maxWait) {
			isConnected, consecutiveFails, lastError = poller.GetState()
			if isConnected && consecutiveFails == 0 && lastError == nil {
				// Successfully reconnected
				break
			}
			time.Sleep(100 * time.Millisecond)
		}

		// Verify reconnection succeeded
		assert.True(t, isConnected, "Should be reconnected")
		assert.Equal(t, 0, consecutiveFails, "Failure count should reset")
		assert.Nil(t, lastError, "Error should be cleared")
	})

	t.Run("TracksConsecutiveFailures", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		pollCount := 0
		mockClient := &MockCoordinatorClient{
			PollFunc: func(_ context.Context, _ *coordinatorv1.PollRequest, _ ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
				pollCount++
				// Always fail
				return nil, status.Error(codes.Unavailable, "connection refused")
			},
		}

		mockExecutor := &MockTaskExecutor{}
		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run poller in background
		go poller.Run(ctx)

		// Wait and check that failures accumulate
		time.Sleep(100 * time.Millisecond)
		_, failures1, _ := poller.GetState()

		time.Sleep(2 * time.Second)
		_, failures2, _ := poller.GetState()

		assert.Greater(t, failures2, failures1, "Consecutive failures should increase")
	})
}

// TestPollerTaskExecution tests task execution through the poller
func TestPollerTaskExecution(t *testing.T) {
	t.Run("ExecutesReceivedTasks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		taskSent := false
		mockClient := &MockCoordinatorClient{
			PollFunc: func(ctx context.Context, _ *coordinatorv1.PollRequest, _ ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
				if !taskSent {
					taskSent = true
					return &coordinatorv1.PollResponse{
						Task: &coordinatorv1.Task{
							DagRunId: "test-run-123",
						},
					}, nil
				}
				// Block on subsequent polls
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}

		mockExecutor := &MockTaskExecutor{
			ExecutionTime: 50 * time.Millisecond,
		}

		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run poller
		go poller.Run(ctx)

		// Wait for task execution
		time.Sleep(200 * time.Millisecond)

		// Verify task was executed
		mockExecutor.mu.Lock()
		executedTasks := mockExecutor.ExecutedTasks
		mockExecutor.mu.Unlock()

		require.Len(t, executedTasks, 1)
		assert.Equal(t, "test-run-123", executedTasks[0])
	})

	t.Run("HandlesExecutionErrors", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		taskSent := false
		executionError := fmt.Errorf("task execution failed")
		var executedWithError atomic.Bool

		mockClient := &MockCoordinatorClient{
			PollFunc: func(ctx context.Context, _ *coordinatorv1.PollRequest, _ ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
				if !taskSent {
					taskSent = true
					return &coordinatorv1.PollResponse{
						Task: &coordinatorv1.Task{
							DagRunId: "failing-task",
						},
					}, nil
				}
				// Block on subsequent polls
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}

		mockExecutor := &MockTaskExecutor{
			ExecuteFunc: func(_ context.Context, _ *coordinatorv1.Task) error {
				executedWithError.Store(true)
				return executionError
			},
		}

		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run poller
		go poller.Run(ctx)

		// Wait for task execution
		time.Sleep(200 * time.Millisecond)

		// Verify error was handled gracefully
		assert.True(t, executedWithError.Load(), "Task should have been executed even though it failed")

		// Poller should still be running (not crashed)
		isConnected, _, _ := poller.GetState()
		assert.True(t, isConnected, "Poller should remain connected after task execution error")
	})
}

// TestPollerConcurrency tests concurrent access to poller state
func TestPollerConcurrency(t *testing.T) {
	t.Run("ConcurrentStateAccess", func(_ *testing.T) {
		mockClient := &MockCoordinatorClient{}
		mockExecutor := &MockTaskExecutor{}

		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run multiple goroutines accessing state concurrently
		var wg sync.WaitGroup
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 100; j++ {
					isConnected, consecutiveFails, lastError := poller.GetState()
					// Just access the values to ensure no race conditions
					_ = isConnected
					_ = consecutiveFails
					_ = lastError
				}
			}()
		}

		wg.Wait()
	})
}

// TestPollerContextCancellation tests proper shutdown on context cancellation
func TestPollerContextCancellation(t *testing.T) {
	t.Run("StopsOnContextCancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		var pollCount atomic.Int32
		mockClient := &MockCoordinatorClient{
			PollFunc: func(ctx context.Context, _ *coordinatorv1.PollRequest, _ ...grpc.CallOption) (*coordinatorv1.PollResponse, error) {
				pollCount.Add(1)
				// Simulate long polling
				select {
				case <-time.After(100 * time.Millisecond):
					return &coordinatorv1.PollResponse{}, nil
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			},
		}

		mockExecutor := &MockTaskExecutor{}
		poller := worker.NewPoller("test-worker", "localhost:8080", mockClient, mockExecutor, 0, nil)

		// Run poller
		done := make(chan struct{})
		go func() {
			poller.Run(ctx)
			close(done)
		}()

		// Let it poll a few times
		time.Sleep(300 * time.Millisecond)
		initialPollCount := pollCount.Load()

		// Cancel context
		cancel()

		// Wait for poller to stop
		select {
		case <-done:
			// Good, poller stopped
		case <-time.After(2 * time.Second):
			t.Fatal("Poller did not stop after context cancellation")
		}

		// Verify polling stopped
		assert.Greater(t, initialPollCount, int32(0), "Should have polled at least once")
	})
}
