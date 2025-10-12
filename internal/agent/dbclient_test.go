package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/builder"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestDBClient_GetChildDAGRunStatus(t *testing.T) {
	t.Run("BasicCase", func(t *testing.T) {
		ctx := context.Background()

		// Setup mocks
		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)
		mockAttempt := new(mockDAGRunAttempt)

		rootRef := core.NewDAGRunRef("parent-dag", "parent-run-123")
		childRunID := "child-run-123"

		// Setup outputs
		outputs := &core.SyncMap{}
		outputs.Store("key1", "result=success")
		outputs.Store("key2", "count=42")
		mockAttempt.outputs = outputs

		// Setup expectations
		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   status.Success,
			Params:   "param1=value1",
			Nodes: []*models.Node{
				{OutputVariables: outputs},
			},
		}, nil)

		// Create dbClient
		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		// Test GetChildDAGRunStatus
		st, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, st)

		// Verify the status
		assert.Equal(t, "child-dag", st.Name)
		assert.Equal(t, childRunID, st.DAGRunID)
		assert.Equal(t, "param1=value1", st.Params)
		assert.Equal(t, map[string]string{"result": "success", "count": "42"}, st.Outputs)

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)

		rootRef := core.NewDAGRunRef("parent-dag", "parent-run-notfound")
		childRunID := "non-existent-child"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(nil, errors.New("not found"))

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		status, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		assert.Error(t, err)
		assert.Nil(t, status)
		assert.Contains(t, err.Error(), "failed to find run for dag-run ID")

		mockDAGRunStore.AssertExpectations(t)
	})
}

func TestDBClient_IsChildDAGRunCompleted(t *testing.T) {
	t.Run("CompletedWithSuccess", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)
		mockAttempt := new(mockDAGRunAttempt)

		rootRef := core.NewDAGRunRef("parent-dag", "parent-run-completed")
		childRunID := "child-completed-success"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   status.Success,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
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

		rootRef := core.NewDAGRunRef("parent-dag", "parent-run-error")
		childRunID := "child-completed-error"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   status.Error,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
		require.NoError(t, err)
		assert.True(t, completed, "StatusError should be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})
	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(mockDAGStore)
		mockDAGRunStore := new(mockDAGRunStore)

		rootRef := core.NewDAGRunRef("parent-dag", "parent-run-notfound")
		childRunID := "non-existent-child"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(nil, errors.New("not found"))

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
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

func (m *mockDAGStore) List(ctx context.Context, params models.ListDAGsOptions) (models.PaginatedResult[*core.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(models.PaginatedResult[*core.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *mockDAGStore) GetMetadata(ctx context.Context, fileName string) (*core.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...builder.LoadOption) (*core.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGStore) Grep(ctx context.Context, pattern string) ([]*models.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*models.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
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

func (m *mockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...builder.LoadOption) (*core.DAG, error) {
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

var _ models.DAGRunStore = (*mockDAGRunStore)(nil)

// mockDAGRunStore implements models.DAGRunStore
type mockDAGRunStore struct {
	mock.Mock
}

// RemoveDAGRun implements models.DAGRunStore.
func (m *mockDAGRunStore) RemoveDAGRun(ctx context.Context, dagRun core.DAGRunRef) error {
	panic("unimplemented")
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts models.NewDAGRunAttemptOptions) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []models.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]models.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) ListStatuses(ctx context.Context, opts ...models.ListDAGRunStatusesOption) ([]*models.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunStore) FindAttempt(ctx context.Context, dagRun core.DAGRunRef) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) FindChildAttempt(ctx context.Context, rootDAGRun core.DAGRunRef, dagRunID string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, rootDAGRun, dagRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
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
	status  *models.DAGRunStatus
	outputs *core.SyncMap
}

func (m *mockDAGRunAttempt) ID() string {
	return "mock-attempt-id"
}

func (m *mockDAGRunAttempt) Open(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Write(ctx context.Context, status models.DAGRunStatus) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) ReadStatus(ctx context.Context) (*models.DAGRunStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DAGRunStatus), args.Error(1)
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

func (m *mockDAGRunAttempt) GetOutputs() *core.SyncMap {
	if m.outputs == nil {
		m.outputs = &core.SyncMap{}
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
