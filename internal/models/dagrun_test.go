package models_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock implementations for testing

type mockDAGRunStore struct {
	mock.Mock
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
	if args.Get(0) == nil {
		return nil
	}
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

func (m *mockDAGRunStore) FindChildAttempt(ctx context.Context, dagRun core.DAGRunRef, childDAGRunID string) (models.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun, childDAGRunID)
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

type mockDAGRunAttempt struct {
	mock.Mock
}

func (m *mockDAGRunAttempt) ID() string {
	args := m.Called()
	return args.String(0)
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

func (m *mockDAGRunAttempt) Hide(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) Hidden() bool {
	args := m.Called()
	return args.Bool(0)
}

// Tests

func TestListDAGRunStatusesOptions(t *testing.T) {
	from := models.NewUTC(time.Now().Add(-24 * time.Hour))
	to := models.NewUTC(time.Now())
	statuses := []status.Status{status.Success, status.Error}

	opts := models.ListDAGRunStatusesOptions{}

	// Apply options
	models.WithFrom(from)(&opts)
	models.WithTo(to)(&opts)
	models.WithStatuses(statuses)(&opts)
	models.WithExactName("test-dag")(&opts)
	models.WithName("partial-name")(&opts)
	models.WithDAGRunID("run-123")(&opts)

	// Verify options were set correctly
	assert.Equal(t, from, opts.From)
	assert.Equal(t, to, opts.To)
	assert.Equal(t, statuses, opts.Statuses)
	assert.Equal(t, "test-dag", opts.ExactName)
	assert.Equal(t, "partial-name", opts.Name)
	assert.Equal(t, "run-123", opts.DAGRunID)
}

func TestNewDAGRunAttemptOptions(t *testing.T) {
	rootDAGRun := &core.DAGRunRef{
		Name: "root-dag",
		ID:   "root-run-123",
	}

	opts := models.NewDAGRunAttemptOptions{
		RootDAGRun: rootDAGRun,
		Retry:      true,
	}

	assert.Equal(t, rootDAGRun, opts.RootDAGRun)
	assert.True(t, opts.Retry)
}

func TestDAGRunStoreInterface(t *testing.T) {
	ctx := context.Background()
	store := &mockDAGRunStore{}
	dag := &core.DAG{Name: "test-dag"}
	ts := time.Now()
	dagRunID := "run-123"

	// Test CreateAttempt
	mockAttempt := &mockDAGRunAttempt{}
	store.On("CreateAttempt", ctx, dag, ts, dagRunID, mock.Anything).Return(mockAttempt, nil)

	attempt, err := store.CreateAttempt(ctx, dag, ts, dagRunID, models.NewDAGRunAttemptOptions{})
	assert.NoError(t, err)
	assert.Equal(t, mockAttempt, attempt)

	// Test RecentAttempts
	attempts := []models.DAGRunAttempt{mockAttempt}
	store.On("RecentAttempts", ctx, "test-dag", 10).Return(attempts)

	result := store.RecentAttempts(ctx, "test-dag", 10)
	assert.Equal(t, attempts, result)

	// Test LatestAttempt
	store.On("LatestAttempt", ctx, "test-dag").Return(mockAttempt, nil)

	latest, err := store.LatestAttempt(ctx, "test-dag")
	assert.NoError(t, err)
	assert.Equal(t, mockAttempt, latest)

	// Test ListStatuses
	statuses := []*models.DAGRunStatus{
		{Name: "test-dag", Status: status.Success},
	}
	store.On("ListStatuses", ctx, mock.Anything).Return(statuses, nil)

	statusList, err := store.ListStatuses(ctx)
	assert.NoError(t, err)
	assert.Equal(t, statuses, statusList)

	// Test FindAttempt
	dagRun := core.DAGRunRef{Name: "test-dag", ID: "run-123"}
	store.On("FindAttempt", ctx, dagRun).Return(mockAttempt, nil)

	found, err := store.FindAttempt(ctx, dagRun)
	assert.NoError(t, err)
	assert.Equal(t, mockAttempt, found)

	// Test FindChildAttempt
	childDAGRunID := "child-run-456"
	store.On("FindChildAttempt", ctx, dagRun, childDAGRunID).Return(mockAttempt, nil)

	childFound, err := store.FindChildAttempt(ctx, dagRun, childDAGRunID)
	assert.NoError(t, err)
	assert.Equal(t, mockAttempt, childFound)

	// Test RemoveOldDAGRuns
	store.On("RemoveOldDAGRuns", ctx, "test-dag", 30).Return(nil)

	err = store.RemoveOldDAGRuns(ctx, "test-dag", 30)
	assert.NoError(t, err)

	// Test RenameDAGRuns
	store.On("RenameDAGRuns", ctx, "old-name", "new-name").Return(nil)

	err = store.RenameDAGRuns(ctx, "old-name", "new-name")
	assert.NoError(t, err)

	store.AssertExpectations(t)
}

func TestDAGRunAttemptInterface(t *testing.T) {
	ctx := context.Background()
	attempt := &mockDAGRunAttempt{}

	// Test ID
	attempt.On("ID").Return("attempt-123")
	assert.Equal(t, "attempt-123", attempt.ID())

	// Test Open
	attempt.On("Open", ctx).Return(nil)
	err := attempt.Open(ctx)
	assert.NoError(t, err)

	// Test Write
	status := models.DAGRunStatus{
		Name:     "test-dag",
		DAGRunID: "run-123",
		Status:   status.Running,
	}
	attempt.On("Write", ctx, status).Return(nil)
	err = attempt.Write(ctx, status)
	assert.NoError(t, err)

	// Test Close
	attempt.On("Close", ctx).Return(nil)
	err = attempt.Close(ctx)
	assert.NoError(t, err)

	// Test ReadStatus
	attempt.On("ReadStatus", ctx).Return(&status, nil)
	readStatus, err := attempt.ReadStatus(ctx)
	assert.NoError(t, err)
	assert.Equal(t, &status, readStatus)

	// Test ReadDAG
	dag := &core.DAG{Name: "test-dag"}
	attempt.On("ReadDAG", ctx).Return(dag, nil)
	readDAG, err := attempt.ReadDAG(ctx)
	assert.NoError(t, err)
	assert.Equal(t, dag, readDAG)

	// Test RequestCancel
	attempt.On("RequestCancel", ctx).Return(nil)
	err = attempt.RequestCancel(ctx)
	assert.NoError(t, err)

	// Test CancelRequested
	attempt.On("CancelRequested", ctx).Return(true, nil)
	canceled, err := attempt.CancelRequested(ctx)
	assert.NoError(t, err)
	assert.True(t, canceled)

	attempt.AssertExpectations(t)
}

func TestDAGRunStoreErrors(t *testing.T) {
	ctx := context.Background()
	store := &mockDAGRunStore{}

	// Test error cases
	expectedErr := errors.New("store error")

	// CreateAttempt error
	store.On("CreateAttempt", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, expectedErr)
	_, err := store.CreateAttempt(ctx, &core.DAG{}, time.Now(), "run-123", models.NewDAGRunAttemptOptions{})
	assert.Equal(t, expectedErr, err)

	// LatestAttempt error
	store.On("LatestAttempt", ctx, "test-dag").Return(nil, models.ErrDAGRunIDNotFound)
	_, err = store.LatestAttempt(ctx, "test-dag")
	assert.Equal(t, models.ErrDAGRunIDNotFound, err)

	// ListStatuses error
	store.On("ListStatuses", ctx, mock.Anything).Return(nil, expectedErr)
	_, err = store.ListStatuses(ctx)
	assert.Equal(t, expectedErr, err)

	// FindAttempt error
	dagRun := core.DAGRunRef{Name: "test-dag", ID: "run-123"}
	store.On("FindAttempt", ctx, dagRun).Return(nil, models.ErrNoStatusData)
	_, err = store.FindAttempt(ctx, dagRun)
	assert.Equal(t, models.ErrNoStatusData, err)

	store.AssertExpectations(t)
}

func TestDAGRunAttemptErrors(t *testing.T) {
	ctx := context.Background()
	attempt := &mockDAGRunAttempt{}

	expectedErr := errors.New("attempt error")

	// Open error
	attempt.On("Open", ctx).Return(expectedErr)
	err := attempt.Open(ctx)
	assert.Equal(t, expectedErr, err)

	// Write error
	status := models.DAGRunStatus{}
	attempt.On("Write", ctx, status).Return(expectedErr)
	err = attempt.Write(ctx, status)
	assert.Equal(t, expectedErr, err)

	// ReadStatus error
	attempt.On("ReadStatus", ctx).Return(nil, models.ErrNoStatusData)
	_, err = attempt.ReadStatus(ctx)
	assert.Equal(t, models.ErrNoStatusData, err)

	// ReadDAG error
	attempt.On("ReadDAG", ctx).Return(nil, expectedErr)
	_, err = attempt.ReadDAG(ctx)
	assert.Equal(t, expectedErr, err)

	// CancelRequested error
	attempt.On("CancelRequested", ctx).Return(false, expectedErr)
	_, err = attempt.CancelRequested(ctx)
	assert.Equal(t, expectedErr, err)

	attempt.AssertExpectations(t)
}

func TestRemoveOldDAGRunsEdgeCases(t *testing.T) {
	ctx := context.Background()
	store := &mockDAGRunStore{}

	// Test with negative retention days (should not delete anything)
	store.On("RemoveOldDAGRuns", ctx, "test-dag", -1).Return(nil)
	err := store.RemoveOldDAGRuns(ctx, "test-dag", -1)
	assert.NoError(t, err)

	// Test with zero retention days (should delete all except non-final statuses)
	store.On("RemoveOldDAGRuns", ctx, "test-dag", 0).Return(nil)
	err = store.RemoveOldDAGRuns(ctx, "test-dag", 0)
	assert.NoError(t, err)

	// Test with positive retention days
	store.On("RemoveOldDAGRuns", ctx, "test-dag", 30).Return(nil)
	err = store.RemoveOldDAGRuns(ctx, "test-dag", 30)
	assert.NoError(t, err)

	store.AssertExpectations(t)
}

func TestListDAGRunStatusesWithOptions(t *testing.T) {
	ctx := context.Background()
	store := &mockDAGRunStore{}

	// Test with multiple options
	from := models.NewUTC(time.Now().Add(-7 * 24 * time.Hour))
	to := models.NewUTC(time.Now())

	opts := []models.ListDAGRunStatusesOption{
		models.WithFrom(from),
		models.WithTo(to),
		models.WithStatuses([]status.Status{status.Success}),
		models.WithName("test"),
	}

	expectedStatuses := []*models.DAGRunStatus{
		{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   status.Success,
		},
	}

	store.On("ListStatuses", ctx, opts).Return(expectedStatuses, nil)

	statuses, err := store.ListStatuses(ctx, opts...)
	assert.NoError(t, err)
	assert.Equal(t, expectedStatuses, statuses)

	store.AssertExpectations(t)
}
