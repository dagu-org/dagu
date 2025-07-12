package coordinator

import (
	"context"
	"testing"
	"time"

	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestHandler_Poll(t *testing.T) {
	t.Run("PollWithoutPollerID", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		_, err := h.Poll(ctx, &coordinatorv1.PollRequest{
			WorkerId: "worker1",
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, st.Code())
		require.Contains(t, st.Message(), "poller_id is required")
	})

	t.Run("PollAndDispatch", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		// Start polling in a goroutine
		pollDone := make(chan *coordinatorv1.PollResponse)
		pollErr := make(chan error)
		go func() {
			resp, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker1",
				PollerId: "poller1",
			})
			if err != nil {
				pollErr <- err
			} else {
				pollDone <- resp
			}
		}()

		// Give the poller time to register
		time.Sleep(100 * time.Millisecond)

		// Dispatch a task
		task := &coordinatorv1.Task{
			RootDagRunName:   "test-dag",
			RootDagRunId:     "run-123",
			ParentDagRunName: "",
			ParentDagRunId:   "",
			DagRunId:         "run-123",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{
			Task: task,
		})
		require.NoError(t, err)

		// Check that the poller received the task
		select {
		case resp := <-pollDone:
			require.NotNil(t, resp)
			require.NotNil(t, resp.Task)
			require.Equal(t, "test-dag", resp.Task.RootDagRunName)
			require.Equal(t, "run-123", resp.Task.RootDagRunId)
		case err := <-pollErr:
			t.Fatalf("Poll failed: %v", err)
		case <-time.After(1 * time.Second):
			t.Fatal("Poll timed out")
		}
	})

	t.Run("DispatchWithNoWaitingPollers", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-123",
			DagRunId:       "run-123",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{
			Task: task,
		})

		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, st.Code())
		require.Contains(t, st.Message(), "no available workers")
	})

	t.Run("PollContextCancellation", func(t *testing.T) {
		h := NewHandler()
		ctx, cancel := context.WithCancel(context.Background())

		// Start polling
		pollDone := make(chan error)
		go func() {
			_, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker1",
				PollerId: "poller1",
			})
			pollDone <- err
		}()

		// Give the poller time to register
		time.Sleep(100 * time.Millisecond)

		// Cancel the context
		cancel()

		// Check that Poll returns with context error
		select {
		case err := <-pollDone:
			require.Error(t, err)
			require.Equal(t, context.Canceled, err)
		case <-time.After(1 * time.Second):
			t.Fatal("Poll did not return after context cancellation")
		}
	})
}

func TestHandler_Heartbeat(t *testing.T) {
	t.Run("ValidHeartbeat", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		req := &coordinatorv1.HeartbeatRequest{
			WorkerId: "worker1",
			Labels:   map[string]string{"type": "compute"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 5,
				BusyPollers:  2,
				RunningTasks: []*coordinatorv1.RunningTask{
					{
						DagRunId:  "run-123",
						DagName:   "test.yaml",
						StartedAt: time.Now().Unix(),
					},
				},
			},
		}

		resp, err := h.Heartbeat(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	t.Run("MissingWorkerID", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		req := &coordinatorv1.HeartbeatRequest{
			Labels: map[string]string{"type": "compute"},
		}

		_, err := h.Heartbeat(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("HeartbeatUpdatesWorkerInfo", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		// Send heartbeat
		req := &coordinatorv1.HeartbeatRequest{
			WorkerId: "worker1",
			Labels:   map[string]string{"type": "compute", "region": "us-east"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 10,
				BusyPollers:  3,
			},
		}

		_, err := h.Heartbeat(ctx, req)
		require.NoError(t, err)

		// Get workers should return the heartbeat data
		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)

		worker := resp.Workers[0]
		require.Equal(t, "worker1", worker.WorkerId)
		require.Equal(t, map[string]string{"type": "compute", "region": "us-east"}, worker.Labels)
		require.Equal(t, int32(10), worker.TotalPollers)
		require.Equal(t, int32(3), worker.BusyPollers)
		require.Greater(t, worker.LastHeartbeatAt, int64(0))
	})

	t.Run("StaleHeartbeatCleanup", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		// Manually add a stale heartbeat
		h.mu.Lock()
		h.heartbeats["old-worker"] = &heartbeatInfo{
			workerID:        "old-worker",
			labels:          map[string]string{"type": "old"},
			lastHeartbeatAt: time.Now().Add(-40 * time.Second), // 40 seconds old
		}
		h.mu.Unlock()

		// Send a new heartbeat from different worker
		req := &coordinatorv1.HeartbeatRequest{
			WorkerId: "new-worker",
			Labels:   map[string]string{"type": "new"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 5,
			},
		}

		_, err := h.Heartbeat(ctx, req)
		require.NoError(t, err)

		// Old worker should be cleaned up
		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)
		require.Equal(t, "new-worker", resp.Workers[0].WorkerId)
	})
}

func TestHandler_GetWorkers(t *testing.T) {
	t.Run("NoWorkers", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Empty(t, resp.Workers)
	})

	t.Run("WorkersFromHeartbeats", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		// Send heartbeats from multiple workers
		workers := []struct {
			id           string
			totalPollers int32
			busyPollers  int32
			labels       map[string]string
		}{
			{"worker1", 5, 2, map[string]string{"type": "compute"}},
			{"worker2", 10, 7, map[string]string{"type": "storage"}},
			{"worker3", 3, 0, map[string]string{"type": "network"}},
		}

		for _, w := range workers {
			_, err := h.Heartbeat(ctx, &coordinatorv1.HeartbeatRequest{
				WorkerId: w.id,
				Labels:   w.labels,
				Stats: &coordinatorv1.WorkerStats{
					TotalPollers: w.totalPollers,
					BusyPollers:  w.busyPollers,
				},
			})
			require.NoError(t, err)
		}

		// Get workers
		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 3)

		// Verify worker data
		workerMap := make(map[string]*coordinatorv1.WorkerInfo)
		for _, w := range resp.Workers {
			workerMap[w.WorkerId] = w
		}

		for _, expected := range workers {
			actual, ok := workerMap[expected.id]
			require.True(t, ok, "Worker %s not found", expected.id)
			require.Equal(t, expected.labels, actual.Labels)
			require.Equal(t, expected.totalPollers, actual.TotalPollers)
			require.Equal(t, expected.busyPollers, actual.BusyPollers)
			require.Greater(t, actual.LastHeartbeatAt, int64(0))
		}
	})

	t.Run("RunningTasksInHeartbeat", func(t *testing.T) {
		h := NewHandler()
		ctx := context.Background()

		// Send heartbeat with running tasks
		runningTasks := []*coordinatorv1.RunningTask{
			{
				DagRunId:  "run-123",
				DagName:   "etl-pipeline.yaml",
				StartedAt: time.Now().Add(-5 * time.Minute).Unix(),
			},
			{
				DagRunId:  "run-124",
				DagName:   "backup-job.yaml",
				StartedAt: time.Now().Add(-1 * time.Minute).Unix(),
			},
		}

		_, err := h.Heartbeat(ctx, &coordinatorv1.HeartbeatRequest{
			WorkerId: "worker1",
			Labels:   map[string]string{"type": "compute"},
			Stats: &coordinatorv1.WorkerStats{
				TotalPollers: 5,
				BusyPollers:  2,
				RunningTasks: runningTasks,
			},
		})
		require.NoError(t, err)

		// Get workers and verify running tasks
		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)

		worker := resp.Workers[0]
		require.Equal(t, int32(2), worker.BusyPollers)
		require.Len(t, worker.RunningTasks, 2)

		// Verify task details
		for i, task := range worker.RunningTasks {
			require.Equal(t, runningTasks[i].DagRunId, task.DagRunId)
			require.Equal(t, runningTasks[i].DagName, task.DagName)
			require.Equal(t, runningTasks[i].StartedAt, task.StartedAt)
		}
	})

}
