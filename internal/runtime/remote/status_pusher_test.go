package remote

import (
	"context"
	"errors"
	"testing"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/proto/convert"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface check
var _ coordinator.Client = (*mockCoordinatorClient)(nil)

// mockCoordinatorClient is a minimal mock for testing StatusPusher.
// Only ReportStatus is used by StatusPusher; other methods panic if called.
type mockCoordinatorClient struct {
	reportStatusFunc func(context.Context, *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error)
}

func (m *mockCoordinatorClient) ReportStatus(ctx context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
	if m.reportStatusFunc != nil {
		return m.reportStatusFunc(ctx, req)
	}
	return nil, errors.New("ReportStatus not configured")
}

// Stub methods for interface compliance - panic if called unexpectedly
func (m *mockCoordinatorClient) Dispatch(_ context.Context, _ *coordinatorv1.Task) error {
	panic("Dispatch not implemented in mock")
}

func (m *mockCoordinatorClient) Poll(_ context.Context, _ backoff.RetryPolicy, _ *coordinatorv1.PollRequest) (*coordinatorv1.Task, error) {
	panic("Poll not implemented in mock")
}

func (m *mockCoordinatorClient) GetWorkers(_ context.Context) ([]*coordinatorv1.WorkerInfo, error) {
	panic("GetWorkers not implemented in mock")
}

func (m *mockCoordinatorClient) Heartbeat(_ context.Context, _ *coordinatorv1.HeartbeatRequest) (*coordinatorv1.HeartbeatResponse, error) {
	panic("Heartbeat not implemented in mock")
}

func (m *mockCoordinatorClient) StreamLogs(_ context.Context) (coordinatorv1.CoordinatorService_StreamLogsClient, error) {
	panic("StreamLogs not implemented in mock")
}

func (m *mockCoordinatorClient) Metrics() coordinator.Metrics {
	return coordinator.Metrics{}
}

func (m *mockCoordinatorClient) Cleanup(_ context.Context) error {
	return nil
}

func (m *mockCoordinatorClient) GetDAGRunStatus(_ context.Context, _, _ string, _ *exec.DAGRunRef) (*coordinatorv1.GetDAGRunStatusResponse, error) {
	panic("GetDAGRunStatus not implemented in mock")
}

func (m *mockCoordinatorClient) RequestCancel(_ context.Context, _, _ string, _ *exec.DAGRunRef) error {
	panic("RequestCancel not implemented in mock")
}

func TestNewStatusPusher(t *testing.T) {
	t.Parallel()

	client := &mockCoordinatorClient{}
	pusher := NewStatusPusher(client, "worker-123")

	require.NotNil(t, pusher)
	assert.Equal(t, "worker-123", pusher.workerID)
	assert.Equal(t, client, pusher.client)
}

func TestPush(t *testing.T) {
	t.Parallel()

	t.Run("Success", func(t *testing.T) {
		t.Parallel()

		var capturedReq *coordinatorv1.ReportStatusRequest
		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				capturedReq = req
				return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		status := exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		err := pusher.Push(context.Background(), status)

		require.NoError(t, err)
		require.NotNil(t, capturedReq)
		assert.Equal(t, "worker-1", capturedReq.WorkerId)
		assert.NotNil(t, capturedReq.Status)
		assert.NotEmpty(t, capturedReq.Status.JsonData)
		// Verify the JSON contains the expected data
		s := convert.ProtoToDAGRunStatus(capturedReq.Status)
		require.NotNil(t, s)
		assert.Equal(t, "run-123", s.DAGRunID)
	})

	t.Run("Rejected", func(t *testing.T) {
		t.Parallel()

		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				return &coordinatorv1.ReportStatusResponse{
					Accepted: false,
					Error:    "duplicate status",
				}, nil
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		status := exec.DAGRunStatus{Name: "test-dag", DAGRunID: "run-123"}

		err := pusher.Push(context.Background(), status)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status rejected")
		assert.Contains(t, err.Error(), "duplicate status")
	})

	t.Run("RejectedNoMessage", func(t *testing.T) {
		t.Parallel()

		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				return &coordinatorv1.ReportStatusResponse{Accepted: false, Error: ""}, nil
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		err := pusher.Push(context.Background(), exec.DAGRunStatus{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "status rejected")
	})

	t.Run("NilResponse", func(t *testing.T) {
		t.Parallel()

		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				return nil, nil
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		err := pusher.Push(context.Background(), exec.DAGRunStatus{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "nil response")
	})

	t.Run("ClientError", func(t *testing.T) {
		t.Parallel()

		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				return nil, errors.New("connection refused")
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		err := pusher.Push(context.Background(), exec.DAGRunStatus{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to report status")
		assert.Contains(t, err.Error(), "connection refused")
	})

	t.Run("ContextCancelled", func(t *testing.T) {
		t.Parallel()

		client := &mockCoordinatorClient{
			reportStatusFunc: func(ctx context.Context, _ *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				return nil, ctx.Err()
			},
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		pusher := NewStatusPusher(client, "worker-1")
		err := pusher.Push(ctx, exec.DAGRunStatus{})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "context canceled")
	})

	t.Run("ComplexStatus", func(t *testing.T) {
		t.Parallel()

		var capturedReq *coordinatorv1.ReportStatusRequest
		client := &mockCoordinatorClient{
			reportStatusFunc: func(_ context.Context, req *coordinatorv1.ReportStatusRequest) (*coordinatorv1.ReportStatusResponse, error) {
				capturedReq = req
				return &coordinatorv1.ReportStatusResponse{Accepted: true}, nil
			},
		}

		status := exec.DAGRunStatus{
			Name:       "complex-dag",
			DAGRunID:   "run-456",
			AttemptID:  "attempt-1",
			Status:     core.Succeeded,
			WorkerID:   "other-worker",
			PID:        12345,
			StartedAt:  "2024-01-01T00:00:00Z",
			FinishedAt: "2024-01-01T00:05:00Z",
			Params:     "key=value",
			Root:       exec.DAGRunRef{Name: "root", ID: "root-id"},
			Parent:     exec.DAGRunRef{Name: "parent", ID: "parent-id"},
			Nodes: []*exec.Node{
				{
					Step:   core.Step{Name: "step-1"},
					Status: core.NodeSucceeded,
				},
			},
		}

		pusher := NewStatusPusher(client, "worker-1")
		err := pusher.Push(context.Background(), status)

		require.NoError(t, err)
		require.NotNil(t, capturedReq)
		require.NotNil(t, capturedReq.Status)

		// Verify complex fields were converted via JSON
		s := convert.ProtoToDAGRunStatus(capturedReq.Status)
		require.NotNil(t, s)
		assert.Equal(t, "complex-dag", s.Name)
		assert.Equal(t, "attempt-1", s.AttemptID)
		assert.Equal(t, core.Succeeded, s.Status)
		assert.False(t, s.Root.Zero())
		assert.False(t, s.Parent.Zero())
		assert.Len(t, s.Nodes, 1)
	})
}
