package exec_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestEnqueueRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		dag         *core.DAG
		status      *exec.DAGRunStatus
		dagRunID    string
		setupMocks  func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore)
		assertMocks func(t *testing.T, att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore)
		wantErr     string
		wantStatus  core.Status
	}{
		{
			name:     "AlreadyQueued",
			dag:      &core.DAG{Name: "test-dag"},
			status:   &exec.DAGRunStatus{Status: core.Queued},
			dagRunID: "run-1",
			setupMocks: func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				// No mock setup needed — function should return early
			},
			assertMocks: func(t *testing.T, att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				att.AssertNotCalled(t, "Open", mock.Anything)
				qs.AssertNotCalled(t, "Enqueue", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				att.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
			},
			wantStatus: core.Queued,
		},
		{
			name:     "Success",
			dag:      &core.DAG{Name: "test-dag"},
			status:   &exec.DAGRunStatus{Status: core.Failed},
			dagRunID: "run-2",
			setupMocks: func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				att.On("Open", mock.Anything).Return(nil)
				att.On("Close", mock.Anything).Return(nil)
				// Write is called BEFORE Enqueue to prevent race with queue processor
				att.On("Write", mock.Anything, mock.MatchedBy(func(s exec.DAGRunStatus) bool {
					return s.Status == core.Queued &&
						s.TriggerType == core.TriggerTypeRetry &&
						s.QueuedAt != ""
				})).Return(nil)
				qs.On("Enqueue", mock.Anything, "test-dag", exec.QueuePriorityLow,
					exec.NewDAGRunRef("test-dag", "run-2")).Return(nil)
			},
			wantStatus: core.Queued,
		},
		{
			name:     "OpenFails",
			dag:      &core.DAG{Name: "test-dag"},
			status:   &exec.DAGRunStatus{Status: core.Failed},
			dagRunID: "run-3",
			setupMocks: func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				att.On("Open", mock.Anything).Return(errors.New("open error"))
			},
			assertMocks: func(t *testing.T, att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				qs.AssertNotCalled(t, "Enqueue", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
				att.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
			},
			wantErr:    "open attempt",
			wantStatus: core.Failed,
		},
		{
			name:     "WriteFails",
			dag:      &core.DAG{Name: "test-dag"},
			status:   &exec.DAGRunStatus{Status: core.Failed},
			dagRunID: "run-4",
			setupMocks: func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				att.On("Open", mock.Anything).Return(nil)
				att.On("Close", mock.Anything).Return(nil)
				att.On("Write", mock.Anything, mock.Anything).Return(errors.New("write error"))
			},
			assertMocks: func(t *testing.T, att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				// Write fails before Enqueue, so Enqueue should never be called
				qs.AssertNotCalled(t, "Enqueue", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
			wantErr:    "write status",
			wantStatus: core.Failed, // rolled back
		},
		{
			name:     "EnqueueFails",
			dag:      &core.DAG{Name: "test-dag"},
			status:   &exec.DAGRunStatus{Status: core.Failed},
			dagRunID: "run-5",
			setupMocks: func(att *exec.MockDAGRunAttempt, qs *exec.MockQueueStore) {
				att.On("Open", mock.Anything).Return(nil)
				att.On("Close", mock.Anything).Return(nil)
				// First Write succeeds (persists Queued status)
				// Second Write is the best-effort rollback
				att.On("Write", mock.Anything, mock.Anything).Return(nil)
				qs.On("Enqueue", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("enqueue error"))
			},
			wantErr:    "enqueue retry",
			wantStatus: core.Failed, // rolled back
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			att := &exec.MockDAGRunAttempt{}
			qs := &exec.MockQueueStore{}

			tt.setupMocks(att, qs)

			statusBefore := tt.status.Status
			err := exec.EnqueueRetry(ctx, qs, att, tt.dag, tt.status, tt.dagRunID)

			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}

			if tt.wantStatus != 0 {
				assert.Equal(t, tt.wantStatus, tt.status.Status)
			} else {
				// If wantStatus not specified, status should not have changed on error
				assert.Equal(t, statusBefore, tt.status.Status)
			}

			if tt.assertMocks != nil {
				tt.assertMocks(t, att, qs)
			}
			att.AssertExpectations(t)
			qs.AssertExpectations(t)
		})
	}
}
