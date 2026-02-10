package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
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

func newMockWatermarkState(lastTick time.Time) *SchedulerState {
	return &SchedulerState{
		Version:  1,
		LastTick: lastTick,
		DAGs:     make(map[string]DAGWatermark),
	}
}

func newHourlyCatchupDAG(t *testing.T, name string) *core.DAG {
	t.Helper()
	return &core.DAG{
		Name:          name,
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
}

func newTestTickPlanner(store WatermarkStore) (*TickPlanner, chan DAGChangeEvent) {
	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		WatermarkStore: store,
		IsSuspended: func(_ context.Context, _ string) bool {
			return false
		},
		GetLatestStatus: func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{}, nil
		},
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
		Events: eventCh,
	})
	return tp, eventCh
}

func TestTickPlanner_InitNoWatermarkStore(t *testing.T) {
	t.Parallel()
	tp := NewTickPlanner(TickPlannerConfig{})
	err := tp.Init(context.Background(), nil)
	require.NoError(t, err)
}

func TestTickPlanner_InitLoadError(t *testing.T) {
	t.Parallel()
	store := &mockWatermarkStore{loadErr: errors.New("disk error")}
	tp, _ := newTestTickPlanner(store)

	err := tp.Init(context.Background(), nil)
	require.NoError(t, err) // non-fatal
	// Falls back to empty state on load error
	tp.mu.RLock()
	assert.NotNil(t, tp.watermarkState)
	assert.Equal(t, 1, tp.watermarkState.Version)
	tp.mu.RUnlock()
}

func TestTickPlanner_InitWithMissedRuns(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)),
	}
	tp, _ := newTestTickPlanner(store)

	dag := newHourlyCatchupDAG(t, "test-dag")
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	buf, ok := tp.buffers["test-dag"]
	require.True(t, ok)
	// Should have 3 missed: 10:00, 11:00, 12:00
	assert.Equal(t, 3, buf.Len())
}

func TestTickPlanner_Advance(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	tickTime := time.Date(2026, 2, 7, 13, 0, 0, 0, time.UTC)
	tp.Advance(tickTime)

	tp.mu.RLock()
	assert.Equal(t, tickTime, tp.watermarkState.LastTick)
	tp.mu.RUnlock()
	assert.True(t, tp.watermarkDirty.Load())
}

func TestTickPlanner_AdvanceBeforeInit(t *testing.T) {
	t.Parallel()

	tp := NewTickPlanner(TickPlannerConfig{})
	// Init must be called before Advance to set watermarkState.
	// This test verifies Init+Advance works with all-default config.
	require.NoError(t, tp.Init(context.Background(), nil))
	tp.Advance(time.Now())
}

func TestTickPlanner_FlushWritesSnapshot(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	tickTime := time.Date(2026, 2, 7, 13, 0, 0, 0, time.UTC)
	tp.Advance(tickTime)
	tp.Flush(context.Background())

	saved := store.lastSaved()
	require.NotNil(t, saved)
	assert.Equal(t, tickTime, saved.LastTick)
	assert.False(t, tp.watermarkDirty.Load())
}

func TestTickPlanner_FlushRemarksDirtyOnError(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{saveErr: errors.New("write error")}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	tp.Advance(time.Now())
	tp.Flush(context.Background())

	assert.True(t, tp.watermarkDirty.Load())
}

func TestTickPlanner_FlushSkipsWhenClean(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	tp.Flush(context.Background())
	assert.Nil(t, store.lastSaved())
}

func TestTickPlanner_PlanCatchupDispatches(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}
	tp, _ := newTestTickPlanner(store)

	dag := newHourlyCatchupDAG(t, "my-dag")
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	runs := tp.Plan(context.Background(), now)

	// Should have one catchup run (drains one per tick)
	assert.Len(t, runs, 1)
	assert.Equal(t, "my-dag", runs[0].DAG.Name)
	assert.Equal(t, core.TriggerTypeCatchUp, runs[0].TriggerType)
}

func TestTickPlanner_PlanCatchupSkipOverlap(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}

	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		WatermarkStore: store,
		IsSuspended: func(_ context.Context, _ string) bool {
			return false
		},
		GetLatestStatus: func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{}, nil
		},
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
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
		Events: eventCh,
	})

	dag := newHourlyCatchupDAG(t, "skip-dag")
	dag.OverlapPolicy = core.OverlapPolicySkip
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	initialLen := tp.buffers["skip-dag"].Len()

	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	runs := tp.Plan(context.Background(), now)

	// Should have skipped (popped without returning a run)
	assert.Len(t, runs, 0)
	// Buffer should be shorter by one (item was popped/discarded)
	if buf, ok := tp.buffers["skip-dag"]; ok {
		assert.Equal(t, initialLen-1, buf.Len())
	}
}

func TestTickPlanner_PlanLiveRun(t *testing.T) {
	t.Parallel()

	// Create a planner with no catchup, just a live schedule
	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		IsSuspended: func(_ context.Context, _ string) bool {
			return false
		},
		GetLatestStatus: func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{}, nil
		},
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "live-run-id", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
		Events: eventCh,
	})

	dag := &core.DAG{
		Name:     "live-dag",
		Schedule: []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	// Tick at 12:00 — hourly schedule should fire
	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	runs := tp.Plan(context.Background(), now)

	assert.Len(t, runs, 1)
	assert.Equal(t, "live-dag", runs[0].DAG.Name)
	assert.Equal(t, core.TriggerTypeScheduler, runs[0].TriggerType)
}

func TestTickPlanner_PlanSuspendedDAGSkipped(t *testing.T) {
	t.Parallel()

	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		IsSuspended: func(_ context.Context, _ string) bool {
			return true // Always suspended
		},
		GetLatestStatus: func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{}, nil
		},
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-id", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
		Events: eventCh,
	})

	dag := &core.DAG{
		Name:     "suspended-dag",
		Schedule: []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	runs := tp.Plan(context.Background(), now)
	assert.Len(t, runs, 0)
}

func TestTickPlanner_DrainEvents_Added(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, eventCh := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	newDAG := &core.DAG{
		Name:     "new-dag",
		Schedule: []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}

	// Send an Added event
	eventCh <- DAGChangeEvent{
		Type:    DAGChangeAdded,
		DAG:     newDAG,
		DAGName: "new-dag",
	}

	// Drain events
	tp.drainEvents(context.Background())

	// Verify entry was added
	_, ok := tp.entries["new-dag"]
	assert.True(t, ok, "new-dag should be in entries")

	// Watermark should be set for new DAG
	tp.mu.RLock()
	_, hasWM := tp.watermarkState.DAGs["new-dag"]
	tp.mu.RUnlock()
	assert.True(t, hasWM, "watermark should be set for new DAG")
}

func TestTickPlanner_DrainEvents_Deleted(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, eventCh := newTestTickPlanner(store)

	dag := &core.DAG{
		Name:     "del-dag",
		Schedule: []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	// Verify it exists
	_, ok := tp.entries["del-dag"]
	require.True(t, ok)

	// Send a Deleted event
	eventCh <- DAGChangeEvent{
		Type:    DAGChangeDeleted,
		DAGName: "del-dag",
	}

	tp.drainEvents(context.Background())

	// Verify entry was removed
	_, ok = tp.entries["del-dag"]
	assert.False(t, ok, "del-dag should be removed from entries")
}

func TestTickPlanner_DrainEvents_Updated(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)),
	}
	tp, eventCh := newTestTickPlanner(store)

	dag := newHourlyCatchupDAG(t, "upd-dag")
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	// Should have a buffer from init
	_, hasBuf := tp.buffers["upd-dag"]
	require.True(t, hasBuf)

	// Send Updated event with different schedule
	updatedDAG := &core.DAG{
		Name:          "upd-dag",
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{mustParseSchedule(t, "*/30 * * * *")}, // changed schedule
	}

	eventCh <- DAGChangeEvent{
		Type:    DAGChangeUpdated,
		DAG:     updatedDAG,
		DAGName: "upd-dag",
	}

	tp.drainEvents(context.Background())

	// Entry should be updated
	entry, ok := tp.entries["upd-dag"]
	require.True(t, ok)
	assert.Equal(t, "*/30 * * * *", entry.schedules[0].Expression)
}

func TestTickPlanner_ConcurrentFlushAndAdvance(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			tp.Advance(time.Date(2026, 2, 7, 12, i, 0, 0, time.UTC))
		}(i)
		go func() {
			defer wg.Done()
			tp.Flush(context.Background())
		}()
	}
	wg.Wait()
}

func TestTickPlanner_PrunesStaleDAGEntries(t *testing.T) {
	t.Parallel()

	state := newMockWatermarkState(time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC))
	state.DAGs = map[string]DAGWatermark{
		"active-dag":  {LastScheduledTime: time.Date(2026, 2, 7, 8, 0, 0, 0, time.UTC)},
		"deleted-dag": {LastScheduledTime: time.Date(2026, 2, 7, 7, 0, 0, 0, time.UTC)},
		"gone-dag":    {LastScheduledTime: time.Date(2026, 2, 7, 6, 0, 0, 0, time.UTC)},
	}
	store := &mockWatermarkStore{state: state}
	tp, _ := newTestTickPlanner(store)

	dags := []*core.DAG{{Name: "active-dag"}}
	require.NoError(t, tp.Init(context.Background(), dags))

	tp.mu.RLock()
	_, hasActive := tp.watermarkState.DAGs["active-dag"]
	_, hasDeleted := tp.watermarkState.DAGs["deleted-dag"]
	_, hasGone := tp.watermarkState.DAGs["gone-dag"]
	tp.mu.RUnlock()

	assert.True(t, hasActive, "active-dag should remain")
	assert.False(t, hasDeleted, "deleted-dag should be pruned")
	assert.False(t, hasGone, "gone-dag should be pruned")
}

func TestTickPlanner_NilWatermarkStoreFullPath(t *testing.T) {
	t.Parallel()

	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		Events: eventCh,
	})

	require.NoError(t, tp.Init(context.Background(), []*core.DAG{{Name: "any-dag"}}))

	tp.Advance(time.Now())
	tp.Plan(context.Background(), time.Now())
	tp.Flush(context.Background())

	// With noop defaults, watermarkState is always initialized
	tp.mu.RLock()
	assert.NotNil(t, tp.watermarkState)
	tp.mu.RUnlock()
}

func TestTickPlanner_AdvanceUpdatesPerDAGWatermarks(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{}
	tp, _ := newTestTickPlanner(store)
	require.NoError(t, tp.Init(context.Background(), nil))

	scheduledTime := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	tp.lastPlanResult = []PlannedRun{
		{
			DAG:           &core.DAG{Name: "test-dag"},
			RunID:         "run-1",
			ScheduledTime: scheduledTime,
			TriggerType:   core.TriggerTypeScheduler,
		},
	}

	tickTime := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	tp.Advance(tickTime)

	tp.mu.RLock()
	wm, ok := tp.watermarkState.DAGs["test-dag"]
	tp.mu.RUnlock()
	assert.True(t, ok, "per-DAG watermark should be set")
	assert.Equal(t, scheduledTime, wm.LastScheduledTime)
}

func TestTickPlanner_PlanBufferCleansEmpty(t *testing.T) {
	t.Parallel()

	store := &mockWatermarkStore{
		state: newMockWatermarkState(time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC)),
	}
	tp, _ := newTestTickPlanner(store)

	dag := newHourlyCatchupDAG(t, "drain-dag")
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	_, exists := tp.buffers["drain-dag"]
	require.True(t, exists, "buffer should exist after init")

	// Drain all items (1 per Plan call)
	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	for range 10 {
		tp.Plan(context.Background(), now)
	}

	_, exists = tp.buffers["drain-dag"]
	assert.False(t, exists, "empty buffer should be removed from map")
}

func TestTickPlanner_ShouldRunGuardRunning(t *testing.T) {
	t.Parallel()

	eventCh := make(chan DAGChangeEvent, 256)
	tp := NewTickPlanner(TickPlannerConfig{
		IsSuspended: func(_ context.Context, _ string) bool {
			return false
		},
		GetLatestStatus: func(_ context.Context, _ *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{Status: core.Running}, nil
		},
		Dispatch: func(_ context.Context, _ *core.DAG, _ string, _ core.TriggerType) error {
			return nil
		},
		GenRunID: func(_ context.Context) (string, error) {
			return "run-id", nil
		},
		IsRunning: func(_ context.Context, _ *core.DAG) (bool, error) {
			return false, nil
		},
		Clock: func() time.Time {
			return time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
		},
		Events: eventCh,
	})

	dag := &core.DAG{
		Name:     "running-dag",
		Schedule: []core.Schedule{mustParseSchedule(t, "0 * * * *")},
	}
	require.NoError(t, tp.Init(context.Background(), []*core.DAG{dag}))

	now := time.Date(2026, 2, 7, 12, 0, 0, 0, time.UTC)
	runs := tp.Plan(context.Background(), now)
	assert.Len(t, runs, 0, "should not plan run when DAG is already running")
}
