// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewZombieDetector(t *testing.T) {
	t.Parallel()

	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}

	// Test with default interval
	detector := NewZombieDetector(dagRunStore, procStore, 0, 0)
	assert.NotNil(t, detector)
	assert.Equal(t, 45*time.Second, detector.interval)

	// Test with custom interval
	detector = NewZombieDetector(dagRunStore, procStore, 60*time.Second, 0)
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
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		// No running DAGs
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{}, nil)

		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
	})

	t.Run("RunningDAGIsAlive", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		// One running DAG
		runningStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{runningStatus}, nil)

		// Mock attempt
		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		// Mock DAG
		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		// Process is alive
		procRef := exec.DAGRunRef{
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
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		// One running DAG
		runningStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{runningStatus}, nil)

		// Mock attempt
		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		// Mock DAG
		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		// Process is NOT alive (zombie)
		procRef := exec.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// ReadStatus returns the full status from the attempt
		attempt.On("ReadStatus", mock.Anything).Return(runningStatus, nil)

		// Expect status update
		attempt.On("Open", mock.Anything).Return(nil)
		attempt.On("Write", mock.Anything, mock.MatchedBy(func(s exec.DAGRunStatus) bool {
			return s.Status == core.Failed && s.FinishedAt != ""
		})).Return(nil)
		attempt.On("Close", mock.Anything).Return(nil)

		// Expect cleanup of stale proc files for the killed run
		procStore.On("CleanStaleFiles", mock.Anything, dag.ProcGroup(), procRef).Return(nil)

		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ZombieWithNoNodesPopulatesFromDAG", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		// Status has NO nodes (process killed before initial status write)
		runningStatus := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-456",
			Status:   core.Running,
			Nodes:    nil, // empty — the bug scenario
		}
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{runningStatus}, nil)

		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-456")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		// DAG has steps defined
		dag := &core.DAG{
			Name: "test-dag",
			Steps: []core.Step{
				{Name: "step1"},
				{Name: "step2"},
			},
		}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := exec.DAGRunRef{Name: dag.Name, ID: "run-456"}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// ReadStatus returns status with no nodes (process killed before writing)
		attempt.On("ReadStatus", mock.Anything).Return(runningStatus, nil)

		// Expect status write with nodes populated from DAG steps
		attempt.On("Open", mock.Anything).Return(nil)
		attempt.On("Write", mock.Anything, mock.MatchedBy(func(s exec.DAGRunStatus) bool {
			return s.Status == core.Failed &&
				len(s.Nodes) == 2 &&
				s.Nodes[0].Step.Name == "step1" &&
				s.Nodes[1].Step.Name == "step2" &&
				s.Nodes[0].Status == core.NodeFailed &&
				s.Nodes[1].Status == core.NodeFailed
		})).Return(nil)
		attempt.On("Close", mock.Anything).Return(nil)

		// Expect cleanup of stale proc files for the killed run
		procStore.On("CleanStaleFiles", mock.Anything, dag.ProcGroup(), procRef).Return(nil)

		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ErrorListingStatuses", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		// Error listing statuses
		dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus(nil), errors.New("db error"))

		// Should handle error gracefully
		detector.detectAndCleanZombies(ctx)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
	})
}

func TestZombieDetector_Start(t *testing.T) {
	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	detector := NewZombieDetector(dagRunStore, procStore, 50*time.Millisecond, 0)

	ctx, cancel := context.WithCancel(context.Background())

	// Set up expectations
	callCount := atomic.Int32{}
	dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{}, nil).Run(func(_ mock.Arguments) {
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
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return((*exec.MockDAGRunAttempt)(nil), errors.New("not found"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "find attempt")

		dagRunStore.AssertExpectations(t)
	})

	t.Run("ErrorReadingDAG", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
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
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := exec.DAGRunRef{
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
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := exec.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// ReadStatus returns the full status from the attempt
		attempt.On("ReadStatus", mock.Anything).Return(status, nil)

		// Fail to open attempt
		attempt.On("Open", mock.Anything).Return(errors.New("open error"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "update status")

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})

	t.Run("ErrorReadingFullStatus", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		attempt := &exec.MockDAGRunAttempt{}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)

		dag := &core.DAG{Name: "test-dag"}
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		procRef := exec.DAGRunRef{
			Name: dag.Name,
			ID:   "run-123",
		}
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// ReadStatus fails — should return error, not fall back to summary
		attempt.On("ReadStatus", mock.Anything).Return((*exec.DAGRunStatus)(nil), errors.New("corrupted file"))

		err := detector.checkAndCleanZombie(ctx, status)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "read full status")

		// Should NOT have tried to write the truncated summary
		attempt.AssertNotCalled(t, "Open", mock.Anything)
		attempt.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)

		dagRunStore.AssertExpectations(t)
		procStore.AssertExpectations(t)
		attempt.AssertExpectations(t)
	})
}

func TestZombieDetector_failureThreshold(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("KillsOnlyAfterThreshold", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{Name: "test-dag"}
		procRef := exec.DAGRunRef{Name: dag.Name, ID: "run-123"}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-123")
		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-123",
			Status:   core.Running,
		}

		// Create detector with threshold=3
		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 3)

		attempt := &exec.MockDAGRunAttempt{}
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

		// First two calls: below threshold, no kill
		for range 2 {
			err := detector.checkAndCleanZombie(ctx, status)
			assert.NoError(t, err)
		}
		attempt.AssertNotCalled(t, "ReadStatus", mock.Anything)

		// Third call: threshold reached, triggers kill
		attempt.On("ReadStatus", mock.Anything).Return(status, nil)
		attempt.On("Open", mock.Anything).Return(nil)
		attempt.On("Write", mock.Anything, mock.MatchedBy(func(s exec.DAGRunStatus) bool {
			return s.Status == core.Failed
		})).Return(nil)
		attempt.On("Close", mock.Anything).Return(nil)
		procStore.On("CleanStaleFiles", mock.Anything, dag.ProcGroup(), procRef).Return(nil)

		err := detector.checkAndCleanZombie(ctx, status)
		assert.NoError(t, err)
		attempt.AssertCalled(t, "Write", mock.Anything, mock.Anything)
		procStore.AssertCalled(t, "CleanStaleFiles", mock.Anything, dag.ProcGroup(), procRef)
	})

	t.Run("CounterResetsOnRecovery", func(t *testing.T) {
		t.Parallel()

		dag := &core.DAG{Name: "test-dag"}
		procRef := exec.DAGRunRef{Name: dag.Name, ID: "run-456"}
		dagRunRef := exec.NewDAGRunRef("test-dag", "run-456")
		status := &exec.DAGRunStatus{
			Name:     "test-dag",
			DAGRunID: "run-456",
			Status:   core.Running,
		}

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 3)

		attempt := &exec.MockDAGRunAttempt{}
		dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)
		attempt.On("ReadDAG", mock.Anything).Return(dag, nil)

		// Stale for 2 calls
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil).Times(2)
		for range 2 {
			_ = detector.checkAndCleanZombie(ctx, status)
		}
		assert.Equal(t, 2, detector.staleCounters["run-456"])

		// Run recovers — becomes alive
		procStore.ExpectedCalls = nil // clear to re-mock
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(true, nil).Once()
		_ = detector.checkAndCleanZombie(ctx, status)
		assert.Equal(t, 0, detector.staleCounters["run-456"])

		// Stale again for 2 more calls — still below threshold
		procStore.ExpectedCalls = nil
		procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)
		for range 2 {
			_ = detector.checkAndCleanZombie(ctx, status)
		}
		// No kill should have occurred (never reached threshold=3 consecutively)
		attempt.AssertNotCalled(t, "ReadStatus", mock.Anything)
	})

	t.Run("CounterAndMutexPrunedWhenRunCompletes", func(t *testing.T) {
		t.Parallel()

		dagRunStore := &mockDAGRunStore{}
		procStore := &mockProcStore{}
		detector := NewZombieDetector(dagRunStore, procStore, time.Second, 3)

		// Pre-seed a stale counter and a run mutex
		detector.staleCounters["old-run"] = 2
		detector.runMutexes["old-run"] = &sync.Mutex{}

		// detectAndCleanZombies returns no running statuses
		dagRunStore.On("ListStatuses", mock.Anything, mock.Anything).Return([]*exec.DAGRunStatus{}, nil)
		detector.detectAndCleanZombies(ctx)

		// Both counter and mutex should be pruned
		assert.Empty(t, detector.staleCounters)
		assert.Empty(t, detector.runMutexes)
	})
}

func TestZombieDetector_compareAndSetGuard(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	dag := &core.DAG{Name: "test-dag"}
	procRef := exec.DAGRunRef{Name: dag.Name, ID: "run-789"}
	dagRunRef := exec.NewDAGRunRef("test-dag", "run-789")
	status := &exec.DAGRunStatus{
		Name:     "test-dag",
		DAGRunID: "run-789",
		Status:   core.Running,
	}

	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	// threshold=1 so first stale check triggers the kill path
	detector := NewZombieDetector(dagRunStore, procStore, time.Second, 1)

	attempt := &exec.MockDAGRunAttempt{}
	dagRunStore.On("FindAttempt", mock.Anything, dagRunRef).Return(attempt, nil)
	attempt.On("ReadDAG", mock.Anything).Return(dag, nil)
	procStore.On("IsRunAlive", mock.Anything, dag.ProcGroup(), procRef).Return(false, nil)

	// ReadStatus returns a terminal status (Succeeded) — run completed between scan and kill
	completedStatus := &exec.DAGRunStatus{
		Name:     "test-dag",
		DAGRunID: "run-789",
		Status:   core.Succeeded,
	}
	attempt.On("ReadStatus", mock.Anything).Return(completedStatus, nil)

	err := detector.checkAndCleanZombie(ctx, status)
	assert.NoError(t, err)

	// Should NOT have written Failed — the compare-and-set guard prevented it
	attempt.AssertNotCalled(t, "Open", mock.Anything)
	attempt.AssertNotCalled(t, "Write", mock.Anything, mock.Anything)
}

func TestZombieDetector_concurrency(t *testing.T) {
	t.Parallel()

	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	detector := NewZombieDetector(dagRunStore, procStore, 10*time.Millisecond, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Make detectAndCleanZombies take longer than the interval
	slowCallCount := atomic.Int32{}
	dagRunStore.On("ListStatuses", ctx, mock.Anything).Return([]*exec.DAGRunStatus{}, nil).Run(func(_ mock.Arguments) {
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

var _ exec.DAGRunStore = (*mockDAGRunStore)(nil)

// Mock DAGRunStore
type mockDAGRunStore struct {
	mock.Mock
}

func (m *mockDAGRunStore) CreateAttempt(ctx context.Context, dag *core.DAG, ts time.Time, dagRunID string, opts exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	args := m.Called(ctx, dag, ts, dagRunID, opts)
	return args.Get(0).(exec.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RecentAttempts(ctx context.Context, name string, itemLimit int) []exec.DAGRunAttempt {
	args := m.Called(ctx, name, itemLimit)
	return args.Get(0).([]exec.DAGRunAttempt)
}

func (m *mockDAGRunStore) LatestAttempt(ctx context.Context, name string) (exec.DAGRunAttempt, error) {
	args := m.Called(ctx, name)
	return args.Get(0).(exec.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) ListStatuses(ctx context.Context, opts ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	args := m.Called(ctx, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*exec.DAGRunStatus), args.Error(1)
}

func (m *mockDAGRunStore) FindAttempt(ctx context.Context, dagRun exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(exec.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) FindSubAttempt(ctx context.Context, dagRun exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	args := m.Called(ctx, dagRun, subDAGRunID)
	return args.Get(0).(exec.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) CreateSubAttempt(ctx context.Context, rootRef exec.DAGRunRef, subDAGRunID string) (exec.DAGRunAttempt, error) {
	args := m.Called(ctx, rootRef, subDAGRunID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(exec.DAGRunAttempt), args.Error(1)
}

func (m *mockDAGRunStore) RemoveOldDAGRuns(ctx context.Context, name string, retentionDays int, opts ...exec.RemoveOldDAGRunsOption) ([]string, error) {
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

func (m *mockDAGRunStore) RemoveDAGRun(ctx context.Context, dagRun exec.DAGRunRef) error {
	args := m.Called(ctx, dagRun)
	return args.Error(0)
}

var _ exec.ProcStore = (*mockProcStore)(nil)

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

func (m *mockProcStore) Acquire(ctx context.Context, groupName string, dagRun exec.DAGRunRef) (exec.ProcHandle, error) {
	args := m.Called(ctx, groupName, dagRun)
	return args.Get(0).(exec.ProcHandle), args.Error(1)
}

func (m *mockProcStore) CountAlive(ctx context.Context, groupName string) (int, error) {
	args := m.Called(ctx, groupName)
	return args.Int(0), args.Error(1)
}

func (m *mockProcStore) IsRunAlive(ctx context.Context, groupName string, dagRun exec.DAGRunRef) (bool, error) {
	args := m.Called(ctx, groupName, dagRun)
	return args.Bool(0), args.Error(1)
}

func (m *mockProcStore) ListAlive(ctx context.Context, groupName string) ([]exec.DAGRunRef, error) {
	args := m.Called(ctx, groupName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]exec.DAGRunRef), args.Error(1)
}

func (m *mockProcStore) ListAllAlive(ctx context.Context) (map[string][]exec.DAGRunRef, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string][]exec.DAGRunRef), args.Error(1)
}

func (m *mockProcStore) CleanStaleFiles(ctx context.Context, groupName string, dagRun exec.DAGRunRef) error {
	args := m.Called(ctx, groupName, dagRun)
	return args.Error(0)
}
