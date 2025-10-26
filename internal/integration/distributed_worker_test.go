package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerLabelMatching(t *testing.T) {
	t.Run("TaskRoutedToMatchingWorker", func(t *testing.T) {
		// Create coordinator handler
		handler := coordinator.NewHandler()
		ctx := context.Background()

		// Simulate worker with GPU label polling
		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "gpu-worker",
			PollerId: "poller-1",
			Labels: map[string]string{
				"gpu":    "true",
				"memory": "64G",
			},
		}

		// Start polling in background
		pollDone := make(chan *coordinatorv1.Task)
		go func() {
			resp, err := handler.Poll(ctx, pollReq)
			if err == nil && resp.Task != nil {
				pollDone <- resp.Task
			}
			close(pollDone)
		}()

		// Dispatch task with matching selector
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  "test-run-1",
			Target:    "test-dag",
			WorkerSelector: map[string]string{
				"gpu": "true",
			},
		}

		// Wait for worker to register and dispatch successfully
		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "Dispatch should succeed once worker is registered")

		// Verify worker received the task
		select {
		case receivedTask := <-pollDone:
			assert.Equal(t, task.DagRunId, receivedTask.DagRunId)
			assert.Equal(t, task.Target, receivedTask.Target)
		case <-time.After(time.Second):
			t.Fatal("Worker did not receive task within timeout")
		}
	})

	t.Run("TaskNotRoutedToNonMatchingWorker", func(t *testing.T) {
		// Create coordinator handler
		handler := coordinator.NewHandler()
		ctx := context.Background()

		// Simulate CPU-only worker polling
		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "cpu-worker",
			PollerId: "poller-2",
			Labels: map[string]string{
				"cpu-arch": "amd64",
				"memory":   "16G",
			},
		}

		// Start polling in background
		pollDone := make(chan bool)
		go func() {
			resp, _ := handler.Poll(ctx, pollReq)
			pollDone <- resp.Task != nil
		}()

		// Dispatch task requiring GPU (should fail - no matching worker)
		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  "test-run-2",
			Target:    "gpu-task",
			WorkerSelector: map[string]string{
				"gpu": "true",
			},
		}

		// Wait for worker to register, then dispatch should fail with proper error
		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			// We expect an error, but it should be "no workers match" not "no available workers"
			// The latter means no workers are registered yet
			if err != nil {
				return err.Error() != "rpc error: code = FailedPrecondition desc = no available workers"
			}
			return false
		}, 2*time.Second, 10*time.Millisecond, "Worker should register before dispatch")
		assert.Error(t, err) // Should fail as no matching worker

		// Verify worker did not receive the task
		select {
		case received := <-pollDone:
			assert.False(t, received, "CPU worker should not receive GPU task")
		case <-time.After(200 * time.Millisecond):
			// Expected - worker should not receive task
		}
	})

	t.Run("EmptySelectorMatchesAnyWorker", func(t *testing.T) {
		// Create coordinator handler
		handler := coordinator.NewHandler()
		ctx := context.Background()

		// Simulate worker with labels polling
		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "labeled-worker",
			PollerId: "poller-3",
			Labels: map[string]string{
				"region": "us-west-2",
				"type":   "general",
			},
		}

		// Start polling in background
		pollDone := make(chan *coordinatorv1.Task)
		go func() {
			resp, err := handler.Poll(ctx, pollReq)
			if err == nil && resp.Task != nil {
				pollDone <- resp.Task
			}
			close(pollDone)
		}()

		// Dispatch task without selector (can run anywhere)
		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_START,
			DagRunId:       "test-run-3",
			Target:         "general-task",
			WorkerSelector: nil, // No selector - matches any worker
		}

		// Wait for worker to register and dispatch successfully
		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "Dispatch should succeed once worker is registered")

		// Verify worker received the task
		select {
		case receivedTask := <-pollDone:
			assert.Equal(t, task.DagRunId, receivedTask.DagRunId)
		case <-time.After(time.Second):
			t.Fatal("Worker did not receive task within timeout")
		}
	})
}
