package scheduler

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewZombieDetector(t *testing.T) {
	t.Parallel()

	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}

	// Test with default interval
	detector := NewZombieDetector(dagRunStore, procStore, 0)
	assert.NotNil(t, detector)
	assert.Equal(t, 45*time.Second, detector.interval)

	// Test with custom interval
	detector = NewZombieDetector(dagRunStore, procStore, 60*time.Second)
	assert.NotNil(t, detector)
	assert.Equal(t, 60*time.Second, detector.interval)
}

func TestZombieDetector_detectAndCleanZombies(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("NoRunningDAGs", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		// No running DAGs
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus{}, nil)

		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
	})

	t.Run("RunningDAGIsAlive", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		// One running DAG
		runningStatus := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus{runningStatus}, nil)

		// Mock attempt
		attempt := &mockDAGRunAttempt{}
		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		// Mock DAG
		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		// Process is alive
		procRef := execution.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(true, nil)

		detector.detectAndCleanZombies(ctx)

		// Should not update status since process is alive
		attempt.AssertNotCalled(t, "Open", mock.Anything)
		attempt.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
		attempt.AssertNotCalled(t, "Close", mock.Anything)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("RunningDAGIsZombie", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		// One running DAG
		runningStatus := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus{runningStatus}, nil)

		// Mock attempt
		attempt := &mockDAGRunAttempt{}
		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		// Mock DAG
		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		// Process is NOT alive (zombie)
		procRef := execution.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// Expect status update
		attempt.On("Open", mock.Anything).Return(nil)
		attempt.On("Write", mock.Anything, mock.MatchedBy(func(s execution.DAGRunStatus) bool {
			return s.Status == core.Failed && s.FinishedAt != ""
		})).Return(nil)
		attempt.On("Close", mock.Anything).Return(nil)

		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ErrorListingStatuses", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		// Error listing statuses
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus(nil), errors.New("db error"))

		// Should handle error gracefully
		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
	})
}

func TestZombieDetector_Start(t *testing.T) {
	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	detector := NewZombieDetector(dagRunStore, procStore, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	// Set up expectations
	callCount := atomic.Int32{}
	dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus{}, nil).Run(func(_ mock.Arguments) {
		callCount.Add(1)
	})

	// Start detector in background
	go detector.Start(ctx)

	// Wait for at least 2 ticks
	time.Sleep(150 * time.Millisecond)

	// Cancel context to stop
	cancel()

	// Give it time to stop
	time.Sleep(50 * time.Millisecond)

	// Should have been called at least twice
	assert.GreaterOrEqual(t, callCount.Load(), int32(2))

	dagRunStore.AssertExpectations(t)
}

func TestZombieDetector_checkAndCleanZombie_errors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("ErrorFindingAttempt", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return((*mockDAGRunAttempt)(nil), errors.New("not found"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "find attempt")

		dagRunStore.AssertExpectations(t)
	})

	t.Run("ErrorReadingDAG", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &mockDAGRunAttempt{}
		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)
		attempt.On("ReadDAG", mock.Anything).Return((*core.DAG)(nil), errors.New("read error"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read dag")

		dagRunStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ErrorCheckingIfAlive", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &mockDAGRunAttempt{}
		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := execution.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, errors.New("check error"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "check alive")

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ErrorUpdatingStatus", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second)

		status := &execution.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &mockDAGRunAttempt{}
		dagRunRef := execution.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := execution.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// Fail to open attempt
		attempt.On("Open", mock.Anything).Return(errors.New("open error"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update status")

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})
}

func TestZombieDetector_concurrency(t *testing.T) {
	t.Parallel()

	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	detector := NewZombieDetector(dagRunStore, procStore, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make detectAndCleanZombies take longer than the interval
	slowCallCount := atomic.Int32{}
	dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*execution.DAGRunStatus{}, nil).Run(func(_ mock.Arguments) {
		slowCallCount.Add(1)
		time.Sleep(30 * time.Millisecond) // Slower than interval
	})

	// Start detector
	go detector.Start(ctx)

	// Let it run for a while
	time.Sleep(100 * time.Millisecond)

	// Cancel to stop
	cancel()

	// Should have skipped some calls due to concurrency protection
	// With 100ms runtime and 10ms interval, without protection we'd expect ~10 calls
	// With protection, we should see fewer calls
	callCount := slowCallCount.Load()
	t.Logf("Call count: %d", callCount)

	// Should be less than what we'd expect without concurrency protection
	assert.Less(t, callCount, int32(8))
	assert.GreaterOrEqual(t, callCount, int32(2))

	dagRunStore.AssertExpectations(t)
}

var _ execution.DAGRunStore = (*mockDAGRunStore)(nil)

// Mock DAGRunStore
type mockDAGRunStore struct {
	mock.Mock
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts execution.NewDAGRunAttemptOptions) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []execution.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]execution.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
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

func (m *mockDAGRunStore) FindSubAttempt(ctx context.Context, dagRun execution.DAGRunRef, subDAGRunID string) (execution.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun, subDAGRunID)
	return args.Get(0).(execution.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int, opts ...execution.RemoveOldDAGRunsOption) ([]string, error) {
	args := m.Called(ctx, name, retentionDays, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockDAGRunStore) RenameDAGRuns(ctx context.Context, oldName, newName string) error {
	args := m.Called(ctx, oldName, newName)
	return args.Error(0)
}

func (m *mockDAGRunStore) RemoveDAGRun(ctx context.Context, dagRun execution.DAGRunRef) error {
	args := m.Called(ctx, dagRun)
	return args.Error(0)
}

var _ execution.ProcStore = (*mockProcStore)(nil)

// Mock ProcStore
type mockProcStore struct {
	mock.Mock
}

// Lock implements execution.ProcStore.
func (m *mockProcStore) Lock(_ context.Context, _ string) error {
	return nil
}

// CountAliveByDAGName implements models.ProcStore.
func (m *mockProcStore) CountAliveByDAGName(_ context.Context, _, _ string) (int, error) {
	return 0, nil
}

// TryLock implements models.ProcStore.
func (m *mockProcStore) TryLock(_ context.Context, _ string) error {
	return nil
}

// Unlock implements models.ProcStore.
func (m *mockProcStore) Unlock(_ context.Context, _ string) {
}

func (m *mockProcStore) Acquire(ctx context.Context, groupName string, dagRun execution.DAGRunRef) (execution.ProcHandle, error) {
	args := m.Called(ctx, groupName, dagRun)
	return args.Get(0).(execution.ProcHandle), args.Error(1)
}

func (m *mockProcStore) CountAlive(ctx context.Context, groupName string) (int, error) {
	args := m.Called(ctx, groupName)
	return args.Int(0), args.Error(1)
}

func (m *mockProcStore) IsRunAlive(ctx context.Context, groupName string, dagRun execution.DAGRunRef) (bool, error) {
	args := m.Called(ctx, groupName, dagRun)
	return args.Bool(0), args.Error(1)
}

func (m *mockProcStore) ListAlive(ctx context.Context, groupName string) ([]execution.DAGRunRef, error) {
	args := m.Called(ctx, groupName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]execution.DAGRunRef), args.Error(1)
}

func (m *mockProcStore) ListAllAlive(ctx context.Context) (map[string][]execution.DAGRunRef, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]execution.DAGRunRef), args.Error(1)
}

var _ execution.DAGRunAttempt = (*mockDAGRunAttempt)(nil)

// Mock DAGRunAttempt
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
	return args.Get(0).(*execution.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunAttempt) ReadDAG(ctx context.Context) (*core.DAG, error) {
	args := m.Called(ctx)
	return args.Get(0).(*core.DAG), args.Error(1)
}

func (m *mockDAGRunAttempt) Abort(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) IsAborting(ctx context.Context) (bool, error) {
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

func (m *mockDAGRunAttempt) WriteOutputs(ctx context.Context, outputs *execution.DAGRunOutputs) error {
	args := m.Called(ctx, outputs)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) ReadOutputs(ctx context.Context) (*execution.DAGRunOutputs, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.DAGRunOutputs), args.Error(1)
}

func (m *mockDAGRunAttempt) WriteMessages(ctx context.Context, messages *execution.LLMMessages) error {
	args := m.Called(ctx, messages)
	return args.Error(0)
}

func (m *mockDAGRunAttempt) ReadMessages(ctx context.Context) (*execution.LLMMessages, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*execution.LLMMessages), args.Error(1)
}
