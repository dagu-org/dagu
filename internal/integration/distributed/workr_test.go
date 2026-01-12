package distributed_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Worker Routing Tests
// =============================================================================
// These tests verify worker label matching and task routing.

func TestWorkerRouting_ExactLabelMatch(t *testing.T) {
	t.Run("taskRoutedToMatchingWorker", func(t *testing.T) {
		handler := coordinator.NewHandler()
		ctx := context.Background()

		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "gpu-worker",
			PollerId: "poller-1",
			Labels: map[string]string{
				"gpu":    "true",
				"memory": "64G",
			},
		}

		pollDone := make(chan *coordinatorv1.Task)
		go func() {
			resp, err := handler.Poll(ctx, pollReq)
			if err == nil && resp.Task != nil {
				pollDone <- resp.Task
			}
			close(pollDone)
		}()

		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  "test-run-1",
			Target:    "test-dag",
			WorkerSelector: map[string]string{
				"gpu": "true",
			},
		}

		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "Dispatch should succeed once worker is registered")

		select {
		case receivedTask := <-pollDone:
			assert.Equal(t, task.DagRunId, receivedTask.DagRunId)
			assert.Equal(t, task.Target, receivedTask.Target)
		case <-time.After(time.Second):
			t.Fatal("Worker did not receive task within timeout")
		}
	})
}

func TestWorkerRouting_NoMatchingWorker(t *testing.T) {
	t.Run("taskNotRoutedToNonMatchingWorker", func(t *testing.T) {
		handler := coordinator.NewHandler()
		ctx := context.Background()

		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "cpu-worker",
			PollerId: "poller-2",
			Labels: map[string]string{
				"cpu-arch": "amd64",
				"memory":   "16G",
			},
		}

		pollDone := make(chan bool)
		go func() {
			resp, _ := handler.Poll(ctx, pollReq)
			pollDone <- resp.Task != nil
		}()

		task := &coordinatorv1.Task{
			Operation: coordinatorv1.Operation_OPERATION_START,
			DagRunId:  "test-run-2",
			Target:    "gpu-task",
			WorkerSelector: map[string]string{
				"gpu": "true",
			},
		}

		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			if err != nil {
				return err.Error() != "rpc error: code = FailedPrecondition desc = no available workers"
			}
			return false
		}, 2*time.Second, 10*time.Millisecond, "Worker should register before dispatch")
		assert.Error(t, err)

		select {
		case received := <-pollDone:
			assert.False(t, received, "CPU worker should not receive GPU task")
		case <-time.After(200 * time.Millisecond):
		}
	})
}

func TestWorkerRouting_EmptySelector(t *testing.T) {
	t.Run("emptySelectorMatchesAnyWorker", func(t *testing.T) {
		handler := coordinator.NewHandler()
		ctx := context.Background()

		pollReq := &coordinatorv1.PollRequest{
			WorkerId: "labeled-worker",
			PollerId: "poller-3",
			Labels: map[string]string{
				"region": "us-west-2",
				"type":   "general",
			},
		}

		pollDone := make(chan *coordinatorv1.Task)
		go func() {
			resp, err := handler.Poll(ctx, pollReq)
			if err == nil && resp.Task != nil {
				pollDone <- resp.Task
			}
			close(pollDone)
		}()

		task := &coordinatorv1.Task{
			Operation:      coordinatorv1.Operation_OPERATION_START,
			DagRunId:       "test-run-3",
			Target:         "general-task",
			WorkerSelector: nil,
		}

		dispatchReq := &coordinatorv1.DispatchRequest{Task: task}
		var err error
		require.Eventually(t, func() bool {
			_, err = handler.Dispatch(ctx, dispatchReq)
			return err == nil
		}, 2*time.Second, 10*time.Millisecond, "Dispatch should succeed once worker is registered")

		select {
		case receivedTask := <-pollDone:
			assert.Equal(t, task.DagRunId, receivedTask.DagRunId)
		case <-time.After(time.Second):
			t.Fatal("Worker did not receive task within timeout")
		}
	})
}
