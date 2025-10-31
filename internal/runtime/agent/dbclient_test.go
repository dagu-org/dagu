package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/common/collections"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDBClient_GetSubDAGRunStatus(t *testing.T) {
	t.Run("BasicCase", func(t *testing.T) {
		ctx := context.Background()

		// Setup mocks
		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)
		mockAttempt := new(mockDAGRunAttempt)

		rootRef := execution.NewDAGRunRef("parent-dag", "parent-run-123")
		subRunID := "child-run-123"

		// Setup outputs
		outputs := &collections.SyncMap{}
		outputs.Store("key1", "result=success")
		outputs.Store("key2", "count=42")
		mockAttempt.outputs = outputs

		// Setup expectations
		mockDAGRunStore.On("FindSubAttempt", ctx, rootRef, subRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&execution.DAGRunStatus{
			Name:     "sub-dag",
			DAGRunID: subRunID,
			Status:   core.Succeeded,
			Params:   "param1=value1",
			Nodes: []*execution.Node{
				{OutputVariables: outputs},
			},
		}, nil)

		// Create dbClient
		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		// Test GetSubDAGRunStatus
		st, err := dbClient.GetSubDAGRunStatus(ctx, subRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Verify the status
		assert.Equal(t, "sub-dag", st.Name)
		assert.Equal(t, subRunID, st.DAGRunID)
		assert.Equal(t, "param1=value1", st.Params)
		assert.Equal(t, map[string]string{"result": "success", "count": "42"}, st.Outputs)

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)

		rootRef := execution.NewDAGRunRef("parent-dag", "parent-run-notfound")
		subRunID := "non-existent-child"

		mockDAGRunStore.On("FindSubAttempt", ctx, rootRef, subRunID).Return(nil, errors.New("not found"))

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		status, err := dbClient.GetSubDAGRunStatus(ctx, subRunID, rootRef)
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "failed to find run for dag-run ID")

		mockDAGRunStore.AssertExpectations(t)
	})
}

func TestDBClient_IsSubDAGRunCompleted(t *testing.T) {
	t.Run("CompletedWithSuccess", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)
		mockAttempt := new(mockDAGRunAttempt)

		rootRef := execution.NewDAGRunRef("parent-dag", "parent-run-completed")
		subRunID := "child-completed-success"

		mockDAGRunStore.On("FindSubAttempt", ctx, rootRef, subRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&execution.DAGRunStatus{
			Name:     "sub-dag",
			DAGRunID: subRunID,
			Status:   core.Succeeded,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsSubDAGRunCompleted(ctx, subRunID, rootRef)
		require.NoError(t, err)
		assert.True(t, completed, "StatusSuccess should be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("CompletedWithError", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)
		mockAttempt := new(mockDAGRunAttempt)

		rootRef := execution.NewDAGRunRef("parent-dag", "parent-run-error")
		subRunID := "child-completed-error"

		mockDAGRunStore.On("FindSubAttempt", ctx, rootRef, subRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&execution.DAGRunStatus{
			Name:     "sub-dag",
			DAGRunID: subRunID,
			Status:   core.Failed,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsSubDAGRunCompleted(ctx, subRunID, rootRef)
		require.NoError(t, err)
		assert.True(t, completed, "StatusError should be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})
	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)

		rootRef := execution.NewDAGRunRef("parent-dag", "parent-run-notfound")
		subRunID := "non-existent-child"

		mockDAGRunStore.On("FindSubAttempt", ctx, rootRef, subRunID).Return(nil, errors.New("not found"))

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsSubDAGRunCompleted(ctx, subRunID, rootRef)
		assert.Error(t, err)
		assert.False(t, completed)
		assert.Contains(t, err.Error(), "failed to find run for dag-run ID")

		mockDAGRunStore.AssertExpectations(t)
	})
}

// mockDAGStore implements models.DAGStore
type mockDAGStore struct {
	mock.Mock
}

func (m *mockDAGStore) Create(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) Delete(ctx context.Context, fileName string) error {
	args := m.Called(ctx, fileName)
	return args.Error(0)
}

func (m *mockDAGStore) List(ctx context.Context, params execution.ListDAGsOptions) (execution.PaginatedResult[*core.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(execution.PaginatedResult[*core.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, fileName string) (*core.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*execution.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*execution.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) Rename(ctx context.Context, oldID, newID string) error {
	args := m.Called(ctx, oldID, newID)
	return args.Error(0)
}

func (m *mockDAGStore) GetSpec(ctx context.Context, fileName string) (string, error) {
	args := m.Called(ctx, fileName)
	return args.Get(0).(string), args.Error(1)
}

func (m *mockDAGStore) UpdateSpec(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...spec.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) TagList(ctx context.Context) ([]string, []string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) ToggleSuspend(ctx context.Context, fileName string, suspend bool) error {
	args := m.Called(ctx, fileName, suspend)
	return args.Error(0)
}

func (m *mockDAGStore) IsSuspended(ctx context.Context, fileName string) bool {
	args := m.Called(ctx, fileName)
	return args.Bool(0)
}

var _ execution.DAGRunStore = (*mockDAGRunStore)(nil)

// mockDAGRunStore implements models.DAGRunStore
type mockDAGRunStore struct {
	mock.Mock
}

// RemoveDAGRun implements models.DAGRunStore.
func (m *mockDAGRunStore) RemoveDAGRun(ctx context.Context, dagRun execution.DAGRunRef) error {
	panic("unimplemented")
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts execution.NewDAGRunAttemptOptions) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []execution.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]execution.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) ListStatuses(ctx context.Context, opts ...execution.ListDAGRunStatusesOption) ([]*execution.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*execution.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunStore) FindAttempt(ctx context.Context, dagRun execution.DAGRunRef) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) FindSubAttempt(ctx context.Context, rootDAGRun execution.DAGRunRef, dagRunID string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, rootDAGRun, dagRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int) error {
	args := m.Called(ctx, name, retentionDays)
	return args.Error(0)
}

func (m *mockDAGRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	args := m.Called(ctx, oldName, newName)
	return args.Error(0)
}

// mockDAGRunAttempt implements the needed methods from models.DAGRunAttempt
type mockDAGRunAttempt struct {
	mock.Mock
	status  *execution.DAGRunStatus
	outputs *collections.SyncMap
}

func (m *mockDAGRunAttempt) ID() string {
	return "mock-attempt-id"
}

func (m *mockDAGRunAttempt) Open(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Write(ctx context.Context, status execution.DAGRunStatus) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) ReadStatus(ctx context.Context) (*execution.DAGRunStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunAttempt) ReadDAG(ctx context.Context) (*core.DAG, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGRunAttempt) RequestCancel(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) CancelRequested(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *mockDAGRunAttempt) GetOutputs() *collections.SyncMap {
	if m.outputs == nil {
		m.outputs = &collections.SyncMap{}
	}
	return m.outputs
}

func (m *mockDAGRunAttempt) Hide(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Hidden() bool {
	args := m.Called()
	return args.Bool(0)
}
