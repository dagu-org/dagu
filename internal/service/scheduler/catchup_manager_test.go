package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWatermarkStore struct {
	state   *SchedulerState
	loadErr error
	saveErr error
	mu      sync.Mutex
	saved   []*SchedulerState
}

func (m *mockWatermarkStore) Load(_ context.Context) (*SchedulerState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.state == nil {
		return &SchedulerState{Version: 1, DAGs: make(map[string]DAGWatermark)}, nil
	}
	return m.state, nil
}

func (m *mockWatermarkStore) Save(_ context.Context, state *SchedulerState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved = append(m.saved, state)
	return nil
}

func (m *mockWatermarkStore) lastSaved() *SchedulerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.saved) == 0 {
		return nil
	}
	return m.saved[len(m.saved)-1]
}

func newTestCatchupManager(store WatermarkStore) *CatchupManager {
	return NewCatchupManager(CatchupManagerConfig{
		WatermarkStore: store,
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "test-run-id", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
	})
}

// newMockWatermarkState creates a SchedulerState with the given lastTick and an
// empty DAGs map. Most catchup manager tests share this setup.
func newMockWatermarkState(lastTick time.Time) *SchedulerState {
	return &SchedulerState{
		Version:  1,
		LastTick: lastTick,
		DAGs:     make(map[string]DAGWatermark),
	}
}

// newHourlyCatchupDAG creates a DAG with an hourly schedule ("0 * * * *") and a
// 6-hour catchup window. Uses mustParseSchedule from catchup_test.go.
func newHourlyCatchupDAG(t *testing.T, name string) *core.DAG {
	t.Helper()
	return &core.DAG{
		Name:          name,
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
}

func TestCatchupManager_InitNoWatermarkStore(t *testing.T) {
	t.Parallel()
	cm := NewCatchupManager(CatchupManagerConfig{})
	err := cm.Init(context.Background(), nil)
	require.NoError(t, err)
}

func TestCatchupManager_InitLoadError(t *testing.T) {
	t.Parallel()
	store := &mockWatermarkStore{loadErr: errors.New("disk error")}
	cm := newTestCatchupManager(store)

	err := cm.Init(context.Background(), nil)
	require.NoError(t, err) // non-fatal
	// watermarkState should remain nil
	cm.mu.RLock()
	assert.Nil(t, cm.watermarkState)
	cm.mu.RUnlock()
}

func TestCatchupManager_InitWithMissedRuns(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)),
	}
	cm := newTestCatchupManager(store)

	dag := newHourlyCatchupDAG(t, "test-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	cm.mu.RLock()
	buf, ok := cm.scheduleBuffers["test-dag"]
	cm.mu.RUnlock()

	require.True(t, ok)
	// Should have 3 missed: 10:00, 11:00, 12:00
	assert.Equal(t, 3, buf.Len())
}

func TestCatchupManager_AdvanceWatermark(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	cm := newTestCatchupManager(store)
	require.NoError(t, cm.Init(context.Background(), nil))

	tickTime := time.Date(2026, 2, 7, 13, 0, 0, 0, time.UTC)
	cm.AdvanceWatermark(tickTime)

	cm.mu.RLock()
	assert.Equal(t, tickTime, cm.watermarkState.LastTick)
	cm.mu.RUnlock()
	assert.True(t, cm.watermarkDirty.Load())
}

func TestCatchupManager_AdvanceWatermarkNilState(t *testing.T) {
	t.Parallel()

	cm := NewCatchupManager(CatchupManagerConfig{})
	// Should not panic with nil watermarkState
	cm.AdvanceWatermark(time.Now())
}

func TestCatchupManager_FlushWritesSnapshot(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	cm := newTestCatchupManager(store)
	require.NoError(t, cm.Init(context.Background(), nil))

	tickTime := time.Date(2026, 2, 7, 13, 0, 0, 0, time.UTC)
	cm.AdvanceWatermark(tickTime)
	cm.Flush(context.Background())

	saved := store.lastSaved()
	require.NotNil(t, saved)
	assert.Equal(t, tickTime, saved.LastTick)
	// Should not be dirty after flush
	assert.False(t, cm.watermarkDirty.Load())
}

func TestCatchupManager_FlushRemarksDirtyOnError(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{saveErr: errors.New("write error")}
	cm := newTestCatchupManager(store)
	require.NoError(t, cm.Init(context.Background(), nil))

	cm.AdvanceWatermark(time.Now())
	cm.Flush(context.Background())

	// Should be re-marked dirty after save error
	assert.True(t, cm.watermarkDirty.Load())
}

func TestCatchupManager_FlushSkipsWhenClean(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	cm := newTestCatchupManager(store)
	require.NoError(t, cm.Init(context.Background(), nil))

	// No changes made, flush should be a no-op
	cm.Flush(context.Background())
	assert.Nil(t, store.lastSaved())
}

func TestCatchupManager_RouteToBuffer(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)),
	}
	cm := newTestCatchupManager(store)

	dag := newHourlyCatchupDAG(t, "test-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	// Should route to existing buffer
	routed := cm.RouteToBuffer("test-dag", QueueItem{
		DAG:           dag,
		ScheduledTime: time.Now(),
		TriggerType:   core.TriggerTypeScheduler,
		ScheduleType:  ScheduleTypeStart,
	})
	assert.True(t, routed)

	// Should not route to non-existent buffer
	assert.False(t, cm.RouteToBuffer("other-dag", QueueItem{}))
}

func TestCatchupManager_ProcessBuffersDispatches(t *testing.T) {
	t.Parallel()

	var dispatched []string
	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}

	cm := NewCatchupManager(CatchupManagerConfig{
		WatermarkStore: store,
		Dispatch: func(_ context.Context, dag *core.DAG, runID string, _ core.TriggerType) error {
			dispatched = append(dispatched, dag.Name+":"+runID)
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-1", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
	})

	dag := newHourlyCatchupDAG(t, "my-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	// ProcessBuffers drains one per tick
	cm.ProcessBuffers(context.Background())
	assert.Len(t, dispatched, 1)
	assert.Equal(t, "my-dag:run-1", dispatched[0])
}

func TestCatchupManager_ProcessBuffersSkipOverlap(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}

	var dispatched int
	cm := NewCatchupManager(CatchupManagerConfig{
		WatermarkStore: store,
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			dispatched++
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-1", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return true, nil // DAG is always running
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
	})

	dag := newHourlyCatchupDAG(t, "skip-dag")
	dag.OverlapPolicy = core.OverlapPolicySkip
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	cm.mu.RLock()
	initialLen := cm.scheduleBuffers["skip-dag"].Len()
	cm.mu.RUnlock()

	cm.ProcessBuffers(context.Background())

	// Should have skipped (popped without dispatching)
	assert.Equal(t, 0, dispatched)
	cm.mu.RLock()
	assert.Equal(t, initialLen-1, cm.scheduleBuffers["skip-dag"].Len())
	cm.mu.RUnlock()
}

func TestCatchupManager_ConcurrentFlushAndAdvance(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	cm := newTestCatchupManager(store)
	require.NoError(t, cm.Init(context.Background(), nil))

	// Run concurrent writes and flushes to verify no race
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			cm.AdvanceWatermark(time.Date(2026, 2, 7, 12, i, 0, 0, time.UTC))
		}(i)
		go func() {
			defer wg.Done()
			cm.Flush(context.Background())
		}()
	}
	wg.Wait()
}

func TestCatchupManager_PrunesStaleDAGEntries(t *testing.T) {
	t.Parallel()

	state := newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC))
	state.DAGs = map[string]DAGWatermark{
		"active-dag":  {LastScheduledTime: time.Date(2026, 2, 7, 8, 0, 0, 0, time.UTC)},
		"deleted-dag": {LastScheduledTime: time.Date(2026, 2, 7, 7, 0, 0, 0, time.UTC)},
		"gone-dag":    {LastScheduledTime: time.Date(2026, 2, 7, 6, 0, 0, 0, time.UTC)},
	}
	store := &mockWatermarkStore{state: state}
	cm := newTestCatchupManager(store)

	// Only "active-dag" still exists
	dags := []*core.DAG{{Name: "active-dag"}}
	require.NoError(t, cm.Init(context.Background(), dags))

	cm.mu.RLock()
	_, hasActive := cm.watermarkState.DAGs["active-dag"]
	_, hasDeleted := cm.watermarkState.DAGs["deleted-dag"]
	_, hasGone := cm.watermarkState.DAGs["gone-dag"]
	cm.mu.RUnlock()

	assert.True(t, hasActive, "active-dag should remain")
	assert.False(t, hasDeleted, "deleted-dag should be pruned")
	assert.False(t, hasGone, "gone-dag should be pruned")
}

func TestCatchupManager_RouteToBufferFullFallback(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}
	cm := newTestCatchupManager(store)

	dag := newHourlyCatchupDAG(t, "test-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	// Set the buffer max to a tiny size
	cm.mu.Lock()
	cm.scheduleBuffers["test-dag"].maxItems = 2
	// Clear existing items
	cm.scheduleBuffers["test-dag"].items = nil
	cm.mu.Unlock()

	// First two should succeed
	assert.True(t, cm.RouteToBuffer("test-dag", QueueItem{DAG: dag, ScheduledTime: time.Now()}))
	assert.True(t, cm.RouteToBuffer("test-dag", QueueItem{DAG: dag, ScheduledTime: time.Now()}))
	// Third should fail (buffer full), returning false so caller dispatches directly
	assert.False(t, cm.RouteToBuffer("test-dag", QueueItem{DAG: dag, ScheduledTime: time.Now()}))
}

func TestCatchupManager_ProcessBuffersCleansEmpty(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}
	cm := newTestCatchupManager(store)

	dag := newHourlyCatchupDAG(t, "drain-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	cm.mu.RLock()
	_, exists := cm.scheduleBuffers["drain-dag"]
	cm.mu.RUnlock()
	require.True(t, exists, "buffer should exist after init")

	// Drain all items (1 per ProcessBuffers call)
	for range 10 {
		cm.ProcessBuffers(context.Background())
	}

	// Empty buffer should have been deleted from the map
	cm.mu.RLock()
	_, exists = cm.scheduleBuffers["drain-dag"]
	cm.mu.RUnlock()
	assert.False(t, exists, "empty buffer should be removed from map")
}

func TestCatchupManager_ProcessBuffersDispatchError(t *testing.T) {
	t.Parallel()

	var dispatched int
	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}

	cm := NewCatchupManager(CatchupManagerConfig{
		WatermarkStore: store,
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			dispatched++
			return errors.New("DAG no longer exists")
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-1", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
	})

	dag := newHourlyCatchupDAG(t, "fail-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	cm.mu.RLock()
	initialLen := cm.scheduleBuffers["fail-dag"].Len()
	cm.mu.RUnlock()
	require.Greater(t, initialLen, 0)

	// Drain all items despite dispatch errors
	for range initialLen + 1 {
		cm.ProcessBuffers(context.Background())
	}

	// All items should have been popped and dispatched (even with errors)
	assert.Equal(t, initialLen, dispatched)

	// Buffer should be cleaned up
	cm.mu.RLock()
	_, exists := cm.scheduleBuffers["fail-dag"]
	cm.mu.RUnlock()
	assert.False(t, exists, "buffer should be removed after draining")
}

func TestCatchupManager_ProcessBuffersIsRunningError(t *testing.T) {
	t.Parallel()

	var dispatched int
	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}

	cm := NewCatchupManager(CatchupManagerConfig{
		WatermarkStore: store,
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			dispatched++
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-1", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, errors.New("proc store unavailable")
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
	})

	dag := newHourlyCatchupDAG(t, "err-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	cm.mu.RLock()
	initialLen := cm.scheduleBuffers["err-dag"].Len()
	cm.mu.RUnlock()
	require.Greater(t, initialLen, 0)

	// isRunning errors should default to "not running" and dispatch proceeds
	cm.ProcessBuffers(context.Background())
	assert.Equal(t, 1, dispatched, "should dispatch despite isRunning error")

	// Buffer should have drained by one
	cm.mu.RLock()
	assert.Equal(t, initialLen-1, cm.scheduleBuffers["err-dag"].Len())
	cm.mu.RUnlock()
}

func TestCatchupManager_NilWatermarkStoreFullPath(t *testing.T) {
	t.Parallel()

	cm := NewCatchupManager(CatchupManagerConfig{})

	// Init with nil watermarkStore
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{{Name: "any-dag"}}))

	// All operations should be safe no-ops
	cm.AdvanceWatermark(time.Now())
	cm.ProcessBuffers(context.Background())
	cm.Flush(context.Background())

	// RouteToBuffer should return false (no buffers)
	assert.False(t, cm.RouteToBuffer("any-dag", QueueItem{}))

	// No state should exist
	cm.mu.RLock()
	assert.Nil(t, cm.watermarkState)
	assert.Nil(t, cm.scheduleBuffers)
	cm.mu.RUnlock()
}

func TestCatchupManager_RouteToBufferAfterDrain(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}
	cm := newTestCatchupManager(store)

	dag := newHourlyCatchupDAG(t, "route-dag")
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	// RouteToBuffer should work initially
	assert.True(t, cm.RouteToBuffer("route-dag", QueueItem{DAG: dag, ScheduledTime: time.Now()}))

	// Drain all items
	for range 10 {
		cm.ProcessBuffers(context.Background())
	}

	// After drain + cleanup, RouteToBuffer should return false (buffer gone)
	assert.False(t, cm.RouteToBuffer("route-dag", QueueItem{DAG: dag, ScheduledTime: time.Now()}),
		"after buffer is drained and cleaned up, RouteToBuffer should fall through to direct dispatch")
}
