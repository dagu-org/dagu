package coordinator

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// mockDAGRunStore is a test implementation of execution.DAGRunStore
type mockDAGRunStore struct {
	attempts    map[string]*mockDAGRunAttempt
	subAttempts map[string]*mockDAGRunAttempt // key: rootID:subID
	mu          sync.Mutex
}

func newMockDAGRunStore() *mockDAGRunStore {
	return &mockDAGRunStore{
		attempts:    make(map[string]*mockDAGRunAttempt),
		subAttempts: make(map[string]*mockDAGRunAttempt),
	}
}

func (m *mockDAGRunStore) addSubAttempt(rootRef exec.DAGRunRef, subDAGRunID string, status *exec.DAGRunStatus) *mockDAGRunAttempt {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := &mockDAGRunAttempt{
		status: status,
	}
	key := rootRef.ID + ":" + subDAGRunID
	m.subAttempts[key] = attempt
	return attempt
}

func (m *mockDAGRunStore) addAttempt(ref exec.DAGRunRef, status *exec.DAGRunStatus) *mockDAGRunAttempt {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := &mockDAGRunAttempt{
		status: status,
	}
	m.attempts[ref.ID] = attempt
	return attempt
}

func (m *mockDAGRunStore) addAbortingAttempt(ref exec.DAGRunRef, status *exec.DAGRunStatus) *mockDAGRunAttempt {
	m.mu.Lock()
	defer m.mu.Unlock()
	attempt := &mockDAGRunAttempt{
		status:   status,
		aborting: true,
	}
	m.attempts[ref.ID] = attempt
	return attempt
}

func (m *mockDAGRunStore) FindAttempt(_ context.Context, dagRun exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if attempt, ok := m.attempts[dagRun.ID]; ok {
		return attempt, nil
	}
	return nil, exec.ErrDAGRunIDNotFound
}

// Implement other required interface methods (unused in tests)
// These methods return sentinel errors or panic to make test failures obvious if accidentally called.
func (m *mockDAGRunStore) CreateAttempt(_ context.Context, _ *core.DAG, _ time.Time, _ string, _ exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	panic("CreateAttempt not implemented in mock")
}
func (m *mockDAGRunStore) RecentAttempts(_ context.Context, _ string, _ int) []exec.DAGRunAttempt {
	return nil // Empty slice is valid
}
func (m *mockDAGRunStore) LatestAttempt(_ context.Context, _ string) (exec.DAGRunAttempt, error) {
	return nil, exec.ErrDAGRunIDNotFound
}
func (m *mockDAGRunStore) ListStatuses(_ context.Context, _ ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return nil, nil // Empty list is valid
}
func (m *mockDAGRunStore) FindSubAttempt(_ context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := rootRef.ID + ":" + subDAGRunID
	if attempt, ok := m.subAttempts[key]; ok {
		return attempt, nil
	}
	return nil, exec.ErrDAGRunIDNotFound
}
func (m *mockDAGRunStore) CreateSubAttempt(_ context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := rootRef.ID + ":" + subDAGRunID
	attempt := &mockDAGRunAttempt{
		status: &exec.DAGRunStatus{},
	}
	m.subAttempts[key] = attempt
	return attempt, nil
}
func (m *mockDAGRunStore) RemoveOldDAGRuns(_ context.Context, _ string, _ int, _ ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, nil
}
func (m *mockDAGRunStore) RenameDAGRuns(_ context.Context, _, _ string) error { return nil }
func (m *mockDAGRunStore) RemoveDAGRun(_ context.Context, _ exec.DAGRunRef) error {
	return nil
}

// mockDAGRunAttempt is a test implementation of execution.DAGRunAttempt
type mockDAGRunAttempt struct {
	status                 *exec.DAGRunStatus
	opened                 bool
	closed                 bool
	written                bool
	aborting               bool
	stepMessages           map[string][]exec.LLMMessage // stepName -> messages
	writeStepMessagesError error                        // injected error for WriteStepMessages
	mu                     sync.Mutex
}

func (m *mockDAGRunAttempt) ID() string { return "test-attempt" }
func (m *mockDAGRunAttempt) Open(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.opened = true
	return nil
}
func (m *mockDAGRunAttempt) Write(_ context.Context, s exec.DAGRunStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.status = &s
	m.written = true
	return nil
}
func (m *mockDAGRunAttempt) Close(_ context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}
func (m *mockDAGRunAttempt) ReadStatus(_ context.Context) (*exec.DAGRunStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.status == nil {
		return nil, exec.ErrNoStatusData
	}
	// Return a copy to avoid pointer races
	statusCopy := *m.status
	return &statusCopy, nil
}
func (m *mockDAGRunAttempt) ReadDAG(_ context.Context) (*core.DAG, error) { return nil, nil }
func (m *mockDAGRunAttempt) SetDAG(_ *core.DAG)                           {}
func (m *mockDAGRunAttempt) Abort(_ context.Context) error                { return nil }
func (m *mockDAGRunAttempt) IsAborting(_ context.Context) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.aborting, nil
}
func (m *mockDAGRunAttempt) Hide(_ context.Context) error { return nil }
func (m *mockDAGRunAttempt) Hidden() bool                 { return false }
func (m *mockDAGRunAttempt) WriteOutputs(_ context.Context, _ *exec.DAGRunOutputs) error {
	return nil
}
func (m *mockDAGRunAttempt) ReadOutputs(_ context.Context) (*exec.DAGRunOutputs, error) {
	return nil, nil
}
func (m *mockDAGRunAttempt) WriteStepMessages(_ context.Context, stepName string, messages []exec.LLMMessage) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.writeStepMessagesError != nil {
		return m.writeStepMessagesError
	}
	if m.stepMessages == nil {
		m.stepMessages = make(map[string][]exec.LLMMessage)
	}
	m.stepMessages[stepName] = messages
	return nil
}
func (m *mockDAGRunAttempt) ReadStepMessages(_ context.Context, stepName string) ([]exec.LLMMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stepMessages == nil {
		return nil, nil
	}
	return m.stepMessages[stepName], nil
}

// GetStepMessages returns the messages written for a step (for test assertions)
func (m *mockDAGRunAttempt) GetStepMessages(stepName string) []exec.LLMMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stepMessages == nil {
		return nil
	}
	return m.stepMessages[stepName]
}

// Thread-safe getters for test assertions
func (m *mockDAGRunAttempt) WasOpened() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.opened
}

func (m *mockDAGRunAttempt) WasWritten() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.written
}

func (m *mockDAGRunAttempt) WasClosed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

func TestHandler_Poll(t *testing.T) {
	t.Parallel()

	t.Run("PollWithoutPollerID", func(t *testing.T) {
		t.Parallel()
		h := NewHandler(HandlerConfig{})
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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

		// Wait for poller to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 1
		}, time.Second, 10*time.Millisecond)

		// Dispatch a task
		task := &coordinatorv1.Task{
			RootDagRunName:   "test-dag",
			RootDagRunId:     "run-123",
			ParentDagRunName: "",
			ParentDagRunId:   "",
			DagRunId:         "run-123",
			Definition:       "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:        "default",
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-123",
			DagRunId:       "run-123",
			Definition:     "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:      "default",
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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

		// Wait for poller to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 1
		}, time.Second, 10*time.Millisecond)

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
	t.Parallel()

	t.Run("ValidHeartbeat", func(t *testing.T) {
		t.Parallel()
		h := NewHandler(HandlerConfig{})
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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

		// Trigger zombie detection (this is now done periodically, not on heartbeat)
		h.detectAndCleanupZombies(ctx)

		// Old worker should be cleaned up
		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Workers, 1)
		require.Equal(t, "new-worker", resp.Workers[0].WorkerId)
	})
}

func TestHandler_GetWorkers(t *testing.T) {
	t.Parallel()

	t.Run("NoWorkers", func(t *testing.T) {
		t.Parallel()
		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		resp, err := h.GetWorkers(ctx, &coordinatorv1.GetWorkersRequest{})
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.Empty(t, resp.Workers)
	})

	t.Run("WorkersFromHeartbeats", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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
		t.Parallel()

		h := NewHandler(HandlerConfig{})
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

func TestHandler_ZombieDetection(t *testing.T) {
	t.Parallel()

	t.Run("MarkRunFailedUpdatesStatus", func(t *testing.T) {
		t.Parallel()
		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create a running DAG run
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		initialStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
			Nodes: []*exec.Node{
				{Status: core.NodeRunning},
				{Status: core.NodeSucceeded},
			},
		}
		attempt := store.addAttempt(ref, initialStatus)

		// Mark the run as failed
		h.markRunFailed(ctx, "test-dag", "run-123", "worker crashed")

		// Verify the status was updated
		require.True(t, attempt.WasOpened())
		require.True(t, attempt.WasWritten())
		require.True(t, attempt.WasClosed())

		// Check the status
		status, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Failed, status.Status)
		require.Equal(t, "worker crashed", status.Error)
		require.NotEmpty(t, status.FinishedAt)

		// Check that running node was marked as failed
		require.Equal(t, core.NodeFailed, status.Nodes[0].Status)
		require.Equal(t, "worker crashed", status.Nodes[0].Error)
		// Succeeded node should remain unchanged
		require.Equal(t, core.NodeSucceeded, status.Nodes[1].Status)
	})

	t.Run("MarkRunFailedSkipsCompletedRuns", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an already completed DAG run
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		initialStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Succeeded,
		}
		attempt := store.addAttempt(ref, initialStatus)

		// Try to mark the run as failed
		h.markRunFailed(ctx, "test-dag", "run-123", "worker crashed")

		// Verify no writes occurred (status should remain Succeeded)
		require.False(t, attempt.WasWritten())
		status, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, status.Status)
	})

	t.Run("MarkWorkerTasksFailedWithNoStore", func(t *testing.T) {
		t.Parallel()

		// Handler without dagRunStore
		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		info := &heartbeatInfo{
			workerID: "worker1",
			stats: &coordinatorv1.WorkerStats{
				RunningTasks: []*coordinatorv1.RunningTask{
					{DagRunId: "run-123", DagName: "test-dag"},
				},
			},
		}

		// Should not panic, just skip
		h.markWorkerTasksFailed(ctx, info)
	})

	t.Run("MarkWorkerTasksFailedWithNoStats", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		info := &heartbeatInfo{
			workerID: "worker1",
			stats:    nil, // No stats
		}

		// Should not panic, just skip
		h.markWorkerTasksFailed(ctx, info)
	})

	t.Run("StaleHeartbeatMarksTasksAsFailed", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create a running DAG run
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		initialStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		attempt := store.addAttempt(ref, initialStatus)

		// Add a stale heartbeat with running tasks
		h.mu.Lock()
		h.heartbeats["stale-worker"] = &heartbeatInfo{
			workerID:        "stale-worker",
			lastHeartbeatAt: time.Now().Add(-40 * time.Second), // 40 seconds old
			stats: &coordinatorv1.WorkerStats{
				RunningTasks: []*coordinatorv1.RunningTask{
					{DagRunId: "run-123", DagName: "test-dag"},
				},
			},
		}
		h.mu.Unlock()

		// Trigger zombie detection (this is now done periodically, not on heartbeat)
		h.detectAndCleanupZombies(ctx)

		// Verify the stale worker's task was marked as failed
		require.True(t, attempt.WasWritten())
		status, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Failed, status.Status)
		require.Contains(t, status.Error, "stale-worker")
		require.Contains(t, status.Error, "unresponsive")
	})

	t.Run("DetectAndCleanupZombies", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create two running DAG runs
		ref1 := exec.DAGRunRef{Name: "dag1", ID: "run-1"}
		status1 := &exec.DAGRunStatus{
			Name:     "dag1",
			DAGRunID: "run-1",
			Status:   core.Running,
		}
		attempt1 := store.addAttempt(ref1, status1)

		ref2 := exec.DAGRunRef{Name: "dag2", ID: "run-2"}
		status2 := &exec.DAGRunStatus{
			Name:     "dag2",
			DAGRunID: "run-2",
			Status:   core.Running,
		}
		attempt2 := store.addAttempt(ref2, status2)

		// Add a stale heartbeat with both running tasks
		h.mu.Lock()
		h.heartbeats["crashed-worker"] = &heartbeatInfo{
			workerID:        "crashed-worker",
			lastHeartbeatAt: time.Now().Add(-40 * time.Second),
			stats: &coordinatorv1.WorkerStats{
				RunningTasks: []*coordinatorv1.RunningTask{
					{DagRunId: "run-1", DagName: "dag1"},
					{DagRunId: "run-2", DagName: "dag2"},
				},
			},
		}
		h.mu.Unlock()

		// Run zombie detection
		h.detectAndCleanupZombies(ctx)

		// Verify both tasks were marked as failed
		require.True(t, attempt1.WasWritten())
		require.True(t, attempt2.WasWritten())

		s1, _ := attempt1.ReadStatus(ctx)
		s2, _ := attempt2.ReadStatus(ctx)
		require.Equal(t, core.Failed, s1.Status)
		require.Equal(t, core.Failed, s2.Status)

		// Verify the stale worker was removed
		h.mu.Lock()
		_, exists := h.heartbeats["crashed-worker"]
		h.mu.Unlock()
		require.False(t, exists)
	})

	t.Run("StartZombieDetectorRunsPeriodically", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})

		// Create a running DAG run
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		attempt := store.addAttempt(ref, status)

		// Add a stale heartbeat
		h.mu.Lock()
		h.heartbeats["zombie-worker"] = &heartbeatInfo{
			workerID:        "zombie-worker",
			lastHeartbeatAt: time.Now().Add(-40 * time.Second),
			stats: &coordinatorv1.WorkerStats{
				RunningTasks: []*coordinatorv1.RunningTask{
					{DagRunId: "run-123", DagName: "test-dag"},
				},
			},
		}
		h.mu.Unlock()

		// Start zombie detector with short interval for testing
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		h.StartZombieDetector(ctx, 50*time.Millisecond)

		// Wait for detector to mark task as failed
		require.Eventually(t, func() bool {
			return attempt.WasWritten()
		}, time.Second, 10*time.Millisecond)

		// Verify the task was marked as failed
		s, _ := attempt.ReadStatus(ctx)
		require.Equal(t, core.Failed, s.Status)
	})
}

func TestHandler_ReportStatus(t *testing.T) {
	t.Parallel()

	t.Run("ValidStatusReport", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an attempt for the DAG run
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		})

		// Report status
		protoStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		})
		require.NoError(t, convErr)

		req := &coordinatorv1.ReportStatusRequest{
			Status: protoStatus,
		}

		resp, err := h.ReportStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Accepted)
	})

	t.Run("MissingStatusReturnsError", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		req := &coordinatorv1.ReportStatusRequest{
			Status: nil,
		}

		_, err := h.ReportStatus(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})

	t.Run("NilDAGRunStoreReturnsError", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{}) // No dagRunStore
		ctx := context.Background()

		protoStatus, convErr := convert.DAGRunStatusToProto(&exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		})
		require.NoError(t, convErr)

		req := &coordinatorv1.ReportStatusRequest{
			Status: protoStatus,
		}

		_, err := h.ReportStatus(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("ChatMessagesPersistence", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an attempt for the DAG run
		ref := exec.DAGRunRef{Name: "chat-dag", ID: "chat-run-123"}
		attempt := store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "chat-dag",
			DAGRunID: "chat-run-123",
			Status:   core.Running,
		})

		// Create status with ChatMessages
		statusWithMessages := &exec.DAGRunStatus{
			Name:     "chat-dag",
			DAGRunID: "chat-run-123",
			Status:   core.Running,
			Nodes: []*exec.Node{
				{
					Step:   core.Step{Name: "chat-step"},
					Status: core.NodeSucceeded,
					ChatMessages: []exec.LLMMessage{
						{Role: exec.RoleUser, Content: "Hello!"},
						{Role: exec.RoleAssistant, Content: "Hi there!", Metadata: &exec.LLMMessageMetadata{
							Provider:    "openai",
							Model:       "gpt-4",
							TotalTokens: 10,
						}},
					},
				},
				{
					Step:   core.Step{Name: "no-messages-step"},
					Status: core.NodeSucceeded,
					// No ChatMessages
				},
			},
		}

		protoStatus, convErr := convert.DAGRunStatusToProto(statusWithMessages)
		require.NoError(t, convErr)

		req := &coordinatorv1.ReportStatusRequest{
			Status: protoStatus,
		}

		resp, err := h.ReportStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Accepted)

		// Verify ChatMessages were persisted via WriteStepMessages
		chatStepMessages := attempt.GetStepMessages("chat-step")
		require.Len(t, chatStepMessages, 2)
		assert.Equal(t, exec.RoleUser, chatStepMessages[0].Role)
		assert.Equal(t, "Hello!", chatStepMessages[0].Content)
		assert.Equal(t, exec.RoleAssistant, chatStepMessages[1].Role)
		assert.Equal(t, "Hi there!", chatStepMessages[1].Content)
		require.NotNil(t, chatStepMessages[1].Metadata)
		assert.Equal(t, "openai", chatStepMessages[1].Metadata.Provider)
		assert.Equal(t, "gpt-4", chatStepMessages[1].Metadata.Model)
		assert.Equal(t, 10, chatStepMessages[1].Metadata.TotalTokens)

		// Verify no messages were written for step without ChatMessages
		noMsgStepMessages := attempt.GetStepMessages("no-messages-step")
		assert.Nil(t, noMsgStepMessages)
	})

	t.Run("ChatMessagesPersistence_HandlerNodesFallbackNames", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an existing attempt
		ref := exec.DAGRunRef{Name: "handler-dag", ID: "handler-run-123"}
		attempt := store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "handler-dag",
			DAGRunID: "handler-run-123",
			Status:   core.Running,
		})

		// Create status with handler nodes that have empty step names
		statusWithHandlers := &exec.DAGRunStatus{
			Name:     "handler-dag",
			DAGRunID: "handler-run-123",
			Status:   core.Succeeded,
			// OnInit handler with empty step name - should use "on_init" fallback
			OnInit: &exec.Node{
				Step:   core.Step{}, // Empty name
				Status: core.NodeSucceeded,
				ChatMessages: []exec.LLMMessage{
					{Role: exec.RoleAssistant, Content: "Init completed"},
				},
			},
			// OnSuccess handler with explicit name - should use explicit name
			OnSuccess: &exec.Node{
				Step:   core.Step{Name: "my-success-handler"},
				Status: core.NodeSucceeded,
				ChatMessages: []exec.LLMMessage{
					{Role: exec.RoleAssistant, Content: "Success!"},
				},
			},
		}

		protoStatus, convErr := convert.DAGRunStatusToProto(statusWithHandlers)
		require.NoError(t, convErr)

		req := &coordinatorv1.ReportStatusRequest{
			Status: protoStatus,
		}

		resp, err := h.ReportStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Accepted)

		// Verify OnInit messages were persisted with fallback name "on_init"
		onInitMessages := attempt.GetStepMessages("on_init")
		require.Len(t, onInitMessages, 1)
		assert.Equal(t, "Init completed", onInitMessages[0].Content)

		// Verify OnSuccess messages were persisted with explicit name
		onSuccessMessages := attempt.GetStepMessages("my-success-handler")
		require.Len(t, onSuccessMessages, 1)
		assert.Equal(t, "Success!", onSuccessMessages[0].Content)
	})

	t.Run("ChatMessagesPersistence_WriteErrorDoesNotFailStatus", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an existing attempt with error injection
		ref := exec.DAGRunRef{Name: "error-dag", ID: "error-run-123"}
		attempt := store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "error-dag",
			DAGRunID: "error-run-123",
			Status:   core.Running,
		})

		// Inject error for WriteStepMessages
		attempt.writeStepMessagesError = errors.New("simulated write failure")

		// Create status with ChatMessages
		statusWithMessages := &exec.DAGRunStatus{
			Name:     "error-dag",
			DAGRunID: "error-run-123",
			Status:   core.Succeeded,
			Nodes: []*exec.Node{
				{
					Step:   core.Step{Name: "chat-step"},
					Status: core.NodeSucceeded,
					ChatMessages: []exec.LLMMessage{
						{Role: exec.RoleUser, Content: "Hello!"},
					},
				},
			},
		}

		protoStatus, convErr := convert.DAGRunStatusToProto(statusWithMessages)
		require.NoError(t, convErr)

		req := &coordinatorv1.ReportStatusRequest{
			Status: protoStatus,
		}

		// ReportStatus should succeed even when WriteStepMessages fails
		resp, err := h.ReportStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Accepted)

		// Verify the main status was still written
		require.True(t, attempt.WasWritten())
	})
}

func TestHandler_GetDAGRunStatus(t *testing.T) {
	t.Parallel()

	t.Run("TopLevelDAGLookup", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an attempt with status
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		})

		req := &coordinatorv1.GetDAGRunStatusRequest{
			DagName:  "test-dag",
			DagRunId: "run-123",
		}

		resp, err := h.GetDAGRunStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.True(t, resp.Found)
		require.NotNil(t, resp.Status)
	})

	t.Run("NotFoundReturnsFalse", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		req := &coordinatorv1.GetDAGRunStatusRequest{
			DagName:  "nonexistent-dag",
			DagRunId: "run-999",
		}

		resp, err := h.GetDAGRunStatus(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
		require.False(t, resp.Found)
	})

	t.Run("NilDAGRunStoreReturnsError", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{}) // No dagRunStore
		ctx := context.Background()

		req := &coordinatorv1.GetDAGRunStatusRequest{
			DagName:  "test-dag",
			DagRunId: "run-123",
		}

		_, err := h.GetDAGRunStatus(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("MissingRequiredFieldsReturnsError", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Missing DagName
		req := &coordinatorv1.GetDAGRunStatusRequest{
			DagRunId: "run-123",
		}

		_, err := h.GetDAGRunStatus(ctx, req)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.InvalidArgument, st.Code())
	})
}

func TestMatchesSelector(t *testing.T) {
	t.Parallel()

	t.Run("EmptySelectorMatchesAll", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute", "region": "us-east"}
		selector := map[string]string{}

		require.True(t, matchesSelector(workerLabels, selector))
	})

	t.Run("NilSelectorMatchesAll", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute"}

		require.True(t, matchesSelector(workerLabels, nil))
	})

	t.Run("ExactMatch", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute", "region": "us-east"}
		selector := map[string]string{"type": "compute", "region": "us-east"}

		require.True(t, matchesSelector(workerLabels, selector))
	})

	t.Run("PartialSelectorMatch", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute", "region": "us-east", "tier": "high"}
		selector := map[string]string{"type": "compute"}

		require.True(t, matchesSelector(workerLabels, selector))
	})

	t.Run("PartialSelectorNoMatch", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute"}
		selector := map[string]string{"type": "storage"}

		require.False(t, matchesSelector(workerLabels, selector))
	})

	t.Run("MissingLabelNoMatch", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute"}
		selector := map[string]string{"type": "compute", "region": "us-east"}

		require.False(t, matchesSelector(workerLabels, selector))
	})

	t.Run("EmptyWorkerLabelsWithSelectorNoMatch", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{}
		selector := map[string]string{"type": "compute"}

		require.False(t, matchesSelector(workerLabels, selector))
	})

	t.Run("NilWorkerLabelsWithSelectorNoMatch", func(t *testing.T) {
		t.Parallel()

		selector := map[string]string{"type": "compute"}

		require.False(t, matchesSelector(nil, selector))
	})
}

func TestMatchesNamespaceAffinity(t *testing.T) {
	t.Parallel()

	t.Run("WorkerWithoutNamespaceLabelAcceptsAnyNamespace", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"type": "compute", "region": "us-east"}
		require.True(t, matchesNamespaceAffinity(workerLabels, "team-alpha"))
		require.True(t, matchesNamespaceAffinity(workerLabels, "team-beta"))
		require.True(t, matchesNamespaceAffinity(workerLabels, "default"))
	})

	t.Run("WorkerWithNilLabelsAcceptsAnyNamespace", func(t *testing.T) {
		t.Parallel()

		require.True(t, matchesNamespaceAffinity(nil, "team-alpha"))
		require.True(t, matchesNamespaceAffinity(nil, "default"))
	})

	t.Run("WorkerWithEmptyLabelsAcceptsAnyNamespace", func(t *testing.T) {
		t.Parallel()

		require.True(t, matchesNamespaceAffinity(map[string]string{}, "team-alpha"))
	})

	t.Run("WorkerWithMatchingNamespaceLabelAccepts", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"namespace": "team-alpha", "type": "compute"}
		require.True(t, matchesNamespaceAffinity(workerLabels, "team-alpha"))
	})

	t.Run("WorkerWithDifferentNamespaceLabelRejects", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"namespace": "team-alpha", "type": "compute"}
		require.False(t, matchesNamespaceAffinity(workerLabels, "team-beta"))
	})

	t.Run("WorkerWithDefaultNamespaceLabelAcceptsDefault", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"namespace": "default"}
		require.True(t, matchesNamespaceAffinity(workerLabels, "default"))
		require.False(t, matchesNamespaceAffinity(workerLabels, "team-alpha"))
	})

	t.Run("WorkerWithEmptyNamespaceLabelRejectsNonEmpty", func(t *testing.T) {
		t.Parallel()

		workerLabels := map[string]string{"namespace": ""}
		// Empty namespace label matches only empty task namespace
		require.True(t, matchesNamespaceAffinity(workerLabels, ""))
		require.False(t, matchesNamespaceAffinity(workerLabels, "team-alpha"))
	})
}

func TestHandler_DispatchNamespaceAffinity(t *testing.T) {
	t.Parallel()

	t.Run("DispatchToWorkerWithMatchingNamespaceLabel", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		// Start polling with namespace label
		pollDone := make(chan *coordinatorv1.PollResponse)
		pollErr := make(chan error)
		go func() {
			resp, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker-alpha",
				PollerId: "poller-alpha",
				Labels:   map[string]string{"namespace": "team-alpha"},
			})
			if err != nil {
				pollErr <- err
			} else {
				pollDone <- resp
			}
		}()

		// Wait for poller to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 1
		}, time.Second, 10*time.Millisecond)

		// Dispatch a task in namespace team-alpha
		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-123",
			DagRunId:       "run-123",
			Definition:     "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:      "team-alpha",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{Task: task})
		require.NoError(t, err)

		select {
		case resp := <-pollDone:
			require.NotNil(t, resp)
			require.NotNil(t, resp.Task)
			require.Equal(t, "team-alpha", resp.Task.Namespace)
		case err := <-pollErr:
			t.Fatalf("Poll failed: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Poll timed out")
		}
	})

	t.Run("DispatchSkipsWorkerWithDifferentNamespaceLabel", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		// Start polling with namespace label for team-alpha
		go func() {
			_, _ = h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker-alpha",
				PollerId: "poller-alpha",
				Labels:   map[string]string{"namespace": "team-alpha"},
			})
		}()

		// Wait for poller to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 1
		}, time.Second, 10*time.Millisecond)

		// Dispatch a task in namespace team-beta - should fail since no matching workers
		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-456",
			DagRunId:       "run-456",
			Definition:     "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:      "team-beta",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{Task: task})
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		require.Equal(t, codes.FailedPrecondition, st.Code())
	})

	t.Run("DispatchToWorkerWithoutNamespaceLabelFromAnyNamespace", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{})
		ctx := context.Background()

		// Start polling without namespace label (generic worker)
		pollDone := make(chan *coordinatorv1.PollResponse)
		pollErr := make(chan error)
		go func() {
			resp, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker-generic",
				PollerId: "poller-generic",
				Labels:   map[string]string{"type": "compute"},
			})
			if err != nil {
				pollErr <- err
			} else {
				pollDone <- resp
			}
		}()

		// Wait for poller to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 1
		}, time.Second, 10*time.Millisecond)

		// Dispatch a task from any namespace - generic worker should accept it
		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-789",
			DagRunId:       "run-789",
			Definition:     "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:      "team-alpha",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{Task: task})
		require.NoError(t, err)

		select {
		case resp := <-pollDone:
			require.NotNil(t, resp)
			require.NotNil(t, resp.Task)
			require.Equal(t, "team-alpha", resp.Task.Namespace)
		case err := <-pollErr:
			t.Fatalf("Poll failed: %v", err)
		case <-time.After(time.Second):
			t.Fatal("Poll timed out")
		}
	})

	t.Run("DispatchPrefersNamespaceAffinityWorkerOverGeneric", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{})
		ctx := t.Context()

		// Start two workers:
		// 1. Worker with namespace=team-alpha label
		// 2. Generic worker without namespace label
		affinityDone := make(chan *coordinatorv1.PollResponse, 1)
		go func() {
			resp, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker-alpha",
				PollerId: "poller-alpha",
				Labels:   map[string]string{"namespace": "team-alpha"},
			})
			if err == nil {
				affinityDone <- resp
			}
		}()

		genericDone := make(chan *coordinatorv1.PollResponse, 1)
		go func() {
			resp, err := h.Poll(ctx, &coordinatorv1.PollRequest{
				WorkerId: "worker-generic",
				PollerId: "poller-generic",
				Labels:   map[string]string{"type": "compute"},
			})
			if err == nil {
				genericDone <- resp
			}
		}()

		// Wait for both pollers to register
		require.Eventually(t, func() bool {
			h.mu.Lock()
			defer h.mu.Unlock()
			return len(h.waitingPollers) == 2
		}, time.Second, 10*time.Millisecond)

		// Dispatch a task for team-alpha - both workers can accept it
		task := &coordinatorv1.Task{
			RootDagRunName: "test-dag",
			RootDagRunId:   "run-101",
			DagRunId:       "run-101",
			Definition:     "name: test-dag\nsteps:\n  - name: step1\n    command: echo hello",
			Namespace:      "team-alpha",
		}

		_, err := h.Dispatch(ctx, &coordinatorv1.DispatchRequest{Task: task})
		require.NoError(t, err)

		// One of the workers should receive the task (map iteration is random)
		select {
		case resp := <-affinityDone:
			require.Equal(t, "team-alpha", resp.Task.Namespace)
		case resp := <-genericDone:
			require.Equal(t, "team-alpha", resp.Task.Namespace)
		case <-time.After(time.Second):
			t.Fatal("No worker received the task")
		}
	})
}

func TestCalculateHealthStatus(t *testing.T) {
	t.Parallel()

	t.Run("LessThan5SecondsIsHealthy", func(t *testing.T) {
		t.Parallel()

		status := calculateHealthStatus(0 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY, status)

		status = calculateHealthStatus(4 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_HEALTHY, status)
	})

	t.Run("Between5And15SecondsIsWarning", func(t *testing.T) {
		t.Parallel()

		status := calculateHealthStatus(5 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING, status)

		status = calculateHealthStatus(10 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING, status)

		status = calculateHealthStatus(14 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_WARNING, status)
	})

	t.Run("GreaterThan15SecondsIsUnhealthy", func(t *testing.T) {
		t.Parallel()

		status := calculateHealthStatus(15 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY, status)

		status = calculateHealthStatus(30 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY, status)

		status = calculateHealthStatus(60 * time.Second)
		require.Equal(t, coordinatorv1.WorkerHealthStatus_WORKER_HEALTH_STATUS_UNHEALTHY, status)
	})
}

func TestHandler_Close(t *testing.T) {
	t.Parallel()

	t.Run("ClosesOpenAttempts", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create and cache an attempt
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		attempt := store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		})

		// Manually add to open attempts cache
		h.attemptsMu.Lock()
		h.openAttempts["run-123"] = attempt
		h.attemptsMu.Unlock()

		// Close handler
		h.Close(ctx)

		// Verify attempt was closed
		require.True(t, attempt.WasClosed())

		// Verify cache is cleared
		h.attemptsMu.RLock()
		require.Empty(t, h.openAttempts)
		h.attemptsMu.RUnlock()
	})
}

func TestHandler_GetCancelledRunsForWorker(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsNilWithNilStore", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{}) // No dagRunStore
		ctx := context.Background()

		stats := &coordinatorv1.WorkerStats{
			RunningTasks: []*coordinatorv1.RunningTask{
				{DagRunId: "run-123", DagName: "test-dag"},
			},
		}

		result := h.getCancelledRunsForWorker(ctx, stats)
		require.Nil(t, result)
	})

	t.Run("ReturnsNilWithNilStats", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		result := h.getCancelledRunsForWorker(ctx, nil)
		require.Nil(t, result)
	})

	t.Run("ReturnsNilWithEmptyRunningTasks", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		stats := &coordinatorv1.WorkerStats{
			RunningTasks: []*coordinatorv1.RunningTask{},
		}

		result := h.getCancelledRunsForWorker(ctx, stats)
		require.Nil(t, result)
	})
}

func TestHandlerOptions(t *testing.T) {
	t.Parallel()

	t.Run("WithDAGRunStore", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})

		require.Same(t, store, h.dagRunStore)
	})

	t.Run("WithLogDir", func(t *testing.T) {
		t.Parallel()

		logDir := "/var/log/test"
		h := NewHandler(HandlerConfig{LogDir: logDir})

		require.Equal(t, logDir, h.logDir)
	})

	t.Run("MultipleOptions", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		logDir := "/var/log/test"
		h := NewHandler(HandlerConfig{DAGRunStore: store, LogDir: logDir})

		require.Same(t, store, h.dagRunStore)
		require.Equal(t, logDir, h.logDir)
	})
}

// mockStreamLogsServer implements coordinatorv1.CoordinatorService_StreamLogsServer for testing
type mockStreamLogsServer struct {
	chunks   []*coordinatorv1.LogChunk
	idx      int
	response *coordinatorv1.StreamLogsResponse
	ctx      context.Context
}

func (m *mockStreamLogsServer) Recv() (*coordinatorv1.LogChunk, error) {
	if m.idx >= len(m.chunks) {
		return nil, io.EOF
	}
	chunk := m.chunks[m.idx]
	m.idx++
	return chunk, nil
}

func (m *mockStreamLogsServer) SendAndClose(resp *coordinatorv1.StreamLogsResponse) error {
	m.response = resp
	return nil
}

func (m *mockStreamLogsServer) SetHeader(_ metadata.MD) error  { return nil }
func (m *mockStreamLogsServer) SendHeader(_ metadata.MD) error { return nil }
func (m *mockStreamLogsServer) SetTrailer(_ metadata.MD)       {}
func (m *mockStreamLogsServer) Context() context.Context       { return m.ctx }
func (m *mockStreamLogsServer) SendMsg(_ any) error            { return nil }
func (m *mockStreamLogsServer) RecvMsg(_ any) error            { return nil }

func TestHandler_StreamLogs_Full(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsErrorWhenLogDirEmpty", func(t *testing.T) {
		t.Parallel()

		h := NewHandler(HandlerConfig{}) // No logDir
		stream := &mockStreamLogsServer{
			chunks: []*coordinatorv1.LogChunk{},
			ctx:    context.Background(),
		}

		err := h.StreamLogs(stream)
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.FailedPrecondition, st.Code())
		assert.Contains(t, st.Message(), "logDir is empty")
	})

	t.Run("WritesLogsToFileSystem", func(t *testing.T) {
		t.Parallel()

		logDir := t.TempDir()
		h := NewHandler(HandlerConfig{LogDir: logDir})

		chunks := []*coordinatorv1.LogChunk{
			{
				DagName:    "test-dag",
				DagRunId:   "run-123",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
				Data:       []byte("test log data\n"),
			},
			{
				DagName:    "test-dag",
				DagRunId:   "run-123",
				AttemptId:  "attempt-1",
				StepName:   "step1",
				StreamType: coordinatorv1.LogStreamType_LOG_STREAM_TYPE_STDOUT,
				IsFinal:    true,
			},
		}

		stream := &mockStreamLogsServer{
			chunks: chunks,
			ctx:    context.Background(),
		}

		err := h.StreamLogs(stream)
		require.NoError(t, err)
		require.NotNil(t, stream.response)
		assert.Equal(t, uint64(2), stream.response.ChunksReceived)
		assert.Equal(t, uint64(14), stream.response.BytesWritten)
	})
}

func TestHandler_GetCancelledRunsForWorker_Full(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsCancelledRuns", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an attempt that is aborting (cancelled)
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-123"}
		store.addAbortingAttempt(ref, &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running, // Status doesn't matter, IsAborting is what's checked
		})

		expectedAttemptKey := "test-attempt-key-123"
		stats := &coordinatorv1.WorkerStats{
			RunningTasks: []*coordinatorv1.RunningTask{
				{DagRunId: "run-123", DagName: "test-dag", AttemptKey: expectedAttemptKey},
			},
		}

		result := h.getCancelledRunsForWorker(ctx, stats)
		require.Len(t, result, 1)
		assert.Equal(t, expectedAttemptKey, result[0].AttemptKey)
	})

	t.Run("DoesNotReturnRunningTasks", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Create an attempt that is running (not cancelled)
		ref := exec.DAGRunRef{Name: "test-dag", ID: "run-456"}
		store.addAttempt(ref, &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-456",
			Status:   core.Running,
		})

		stats := &coordinatorv1.WorkerStats{
			RunningTasks: []*coordinatorv1.RunningTask{
				{DagRunId: "run-456", DagName: "test-dag"},
			},
		}

		result := h.getCancelledRunsForWorker(ctx, stats)
		assert.Empty(t, result)
	})
}

func TestHandler_GetOrOpenSubAttempt(t *testing.T) {
	t.Parallel()

	t.Run("OpensSubAttemptOnFirstAccess", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Add a sub-attempt
		rootRef := exec.DAGRunRef{Name: "parent-dag", ID: "root-123"}
		subDAGRunID := "sub-456"
		store.addSubAttempt(rootRef, subDAGRunID, &exec.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: subDAGRunID,
			Status:   core.Running,
		})

		// Get the sub-attempt
		attempt, err := h.getOrOpenSubAttempt(ctx, rootRef, subDAGRunID)
		require.NoError(t, err)
		require.NotNil(t, attempt)

		// Verify it was opened
		mockAttempt := attempt.(*mockDAGRunAttempt)
		assert.True(t, mockAttempt.WasOpened())
	})

	t.Run("ReturnsCachedAttemptOnSecondAccess", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		// Add a sub-attempt
		rootRef := exec.DAGRunRef{Name: "parent-dag", ID: "root-789"}
		subDAGRunID := "sub-101"
		store.addSubAttempt(rootRef, subDAGRunID, &exec.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: subDAGRunID,
			Status:   core.Running,
		})

		// Get the sub-attempt twice
		attempt1, err := h.getOrOpenSubAttempt(ctx, rootRef, subDAGRunID)
		require.NoError(t, err)

		attempt2, err := h.getOrOpenSubAttempt(ctx, rootRef, subDAGRunID)
		require.NoError(t, err)

		// Both should be the same instance
		assert.Same(t, attempt1, attempt2)
	})

	t.Run("ReturnsErrorWhenSubAttemptNotFound", func(t *testing.T) {
		t.Parallel()

		store := newMockDAGRunStore()
		h := NewHandler(HandlerConfig{DAGRunStore: store})
		ctx := context.Background()

		rootRef := exec.DAGRunRef{Name: "parent-dag", ID: "root-999"}

		// Try to get a non-existent sub-attempt
		_, err := h.getOrOpenSubAttempt(ctx, rootRef, "non-existent")
		assert.Error(t, err)
	})
}
