package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockDAGStore implements models.DAGStore
type MockDAGStore struct {
	mock.Mock
}

func (m *MockDAGStore) Create(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *MockDAGStore) Delete(ctx context.Context, fileName string) error {
	args := m.Called(ctx, fileName)
	return args.Error(0)
}

func (m *MockDAGStore) List(ctx context.Context, params models.ListDAGsOptions) (models.PaginatedResult[*digraph.DAG], []string, error) {
	args := m.Called(ctx, params)
	return args.Get(0).(models.PaginatedResult[*digraph.DAG]), args.Get(1).([]string), args.Error(2)
}

func (m *MockDAGStore) GetMetadata(ctx context.Context, fileName string) (*digraph.DAG, error) {
	args := m.Called(ctx, fileName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *MockDAGStore) GetDetails(ctx context.Context, fileName string, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	args := m.Called(ctx, fileName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *MockDAGStore) Grep(ctx context.Context, pattern string) ([]*models.GrepDAGsResult, []string, error) {
	args := m.Called(ctx, pattern)
	return args.Get(0).([]*models.GrepDAGsResult), args.Get(1).([]string), args.Error(2)
}

func (m *MockDAGStore) Rename(ctx context.Context, oldID, newID string) error {
	args := m.Called(ctx, oldID, newID)
	return args.Error(0)
}

func (m *MockDAGStore) GetSpec(ctx context.Context, fileName string) (string, error) {
	args := m.Called(ctx, fileName)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockDAGStore) UpdateSpec(ctx context.Context, fileName string, spec []byte) error {
	args := m.Called(ctx, fileName, spec)
	return args.Error(0)
}

func (m *MockDAGStore) LoadSpec(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	args := m.Called(ctx, spec, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *MockDAGStore) TagList(ctx context.Context) ([]string, []string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Get(1).([]string), args.Error(2)
}

func (m *MockDAGStore) ToggleSuspend(ctx context.Context, fileName string, suspend bool) error {
	args := m.Called(ctx, fileName, suspend)
	return args.Error(0)
}

func (m *MockDAGStore) IsSuspended(ctx context.Context, fileName string) bool {
	args := m.Called(ctx, fileName)
	return args.Bool(0)
}

// MockDAGRunStore implements models.DAGRunStore
type MockDAGRunStore struct {
	mock.Mock
}

func (m *MockDAGRunStore) CreateAttempt(ctx context.Context, dag *digraph.DAG, ts time.Time, dagRunID string, opts models.NewDAGRunAttemptOptions) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *MockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []models.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]models.DAGRunAttempt)
}

func (m *MockDAGRunStore) LatestAttempt(ctx context.Context, name string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *MockDAGRunStore) ListStatuses(ctx context.Context, opts ...models.ListDAGRunStatusesOption) ([]*models.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.DAGRunStatus), args.Error(1)
}

func (m *MockDAGRunStore) FindAttempt(ctx context.Context, dagRun digraph.DAGRunRef) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *MockDAGRunStore) FindChildAttempt(ctx context.Context, rootDAGRun digraph.DAGRunRef, dagRunID string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, rootDAGRun, dagRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(models.DAGRunAttempt), args.Error(1)
}

func (m *MockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int) error {
	args := m.Called(ctx, name, retentionDays)
	return args.Error(0)
}

func (m *MockDAGRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	args := m.Called(ctx, oldName, newName)
	return args.Error(0)
}

// MockDAGRunAttempt implements the needed methods from models.DAGRunAttempt
type MockDAGRunAttempt struct {
	mock.Mock
	status  *models.DAGRunStatus
	outputs *executor.SyncMap
}

func (m *MockDAGRunAttempt) ID() string {
	return "mock-attempt-id"
}

func (m *MockDAGRunAttempt) Open(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) Write(ctx context.Context, status models.DAGRunStatus) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) ReadStatus(ctx context.Context) (*models.DAGRunStatus, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.DAGRunStatus), args.Error(1)
}

func (m *MockDAGRunAttempt) ReadDAG(ctx context.Context) (*digraph.DAG, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*digraph.DAG), args.Error(1)
}

func (m *MockDAGRunAttempt) RequestCancel(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDAGRunAttempt) CancelRequested(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockDAGRunAttempt) GetOutputs() *executor.SyncMap {
	if m.outputs == nil {
		m.outputs = &executor.SyncMap{}
	}
	return m.outputs
}

func TestDBClient_GetChildDAGRunStatus(t *testing.T) {
	t.Run("StatusSuccess", func(t *testing.T) {
		ctx := context.Background()

		// Setup mocks
		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-123")
		childRunID := "child-run-123"

		// Setup outputs
		outputs := &executor.SyncMap{}
		outputs.Store("key1", "result=success")
		outputs.Store("key2", "count=42")
		mockAttempt.outputs = outputs

		// Setup expectations
		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusSuccess,
			Params:   "param1=value1",
			Nodes: []*models.Node{
				{OutputVariables: outputs},
			},
		}, nil)

		// Create dbClient
		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		// Test GetChildDAGRunStatus
		status, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, status)

		// Verify the status
		assert.Equal(t, "child-dag", status.Name)
		assert.Equal(t, childRunID, status.DAGRunID)
		assert.Equal(t, "param1=value1", status.Params)
		assert.True(t, status.Success, "StatusSuccess should result in Success=true")
		assert.Equal(t, map[string]string{"result": "success", "count": "42"}, status.Outputs)

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("StatusPartialSuccess", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-456")
		childRunID := "child-run-456"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusPartialSuccess,
			Params:   "param1=value1",
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		status, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.True(t, status.Success, "StatusPartialSuccess should result in Success=true")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("StatusError", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-789")
		childRunID := "child-run-789"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusError,
			Params:   "param1=value1",
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		status, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.False(t, status.Success, "StatusError should result in Success=false")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("StatusCancel", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-cancel")
		childRunID := "child-run-cancel"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusCancel,
			Params:   "param1=value1",
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		status, err := dbClient.GetChildDAGRunStatus(ctx, childRunID, rootRef)
		require.NoError(t, err)
		require.NotNil(t, status)

		assert.False(t, status.Success, "StatusCancel should result in Success=false")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-notfound")
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

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-completed")
		childRunID := "child-completed-success"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusSuccess,
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

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-error")
		childRunID := "child-completed-error"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusError,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
		require.NoError(t, err)
		assert.True(t, completed, "StatusError should be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("CompletedWithCancel", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-cancel")
		childRunID := "child-completed-cancel"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusCancel,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
		require.NoError(t, err)
		assert.True(t, completed, "StatusCancel should be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("NotCompletedRunning", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-running")
		childRunID := "child-running"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusRunning,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
		require.NoError(t, err)
		assert.False(t, completed, "StatusRunning should not be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("NotCompletedQueued", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)
		mockAttempt := new(MockDAGRunAttempt)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-queued")
		childRunID := "child-queued"

		mockDAGRunStore.On("FindChildAttempt", ctx, rootRef, childRunID).Return(mockAttempt, nil)
		mockAttempt.On("ReadStatus", ctx).Return(&models.DAGRunStatus{
			Name:     "child-dag",
			DAGRunID: childRunID,
			Status:   scheduler.StatusQueued,
		}, nil)

		dbClient := newDBClient(mockDAGRunStore, mockDAGStore)

		completed, err := dbClient.IsChildDAGRunCompleted(ctx, childRunID, rootRef)
		require.NoError(t, err)
		assert.False(t, completed, "StatusQueued should not be completed")

		mockDAGRunStore.AssertExpectations(t)
		mockAttempt.AssertExpectations(t)
	})

	t.Run("ChildNotFound", func(t *testing.T) {
		ctx := context.Background()

		mockDAGStore := new(MockDAGStore)
		mockDAGRunStore := new(MockDAGRunStore)

		rootRef := digraph.NewDAGRunRef("parent-dag", "parent-run-notfound")
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
