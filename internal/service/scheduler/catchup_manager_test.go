package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWatermarkStore struct {
	state   *core.SchedulerState
	loadErr error
	saveErr error
	mu      sync.Mutex
	saved   []*core.SchedulerState
}

func (m *mockWatermarkStore) Load(_ context.Context) (*core.SchedulerState, error) {
	if m.loadErr != nil {
		return nil, m.loadErr
	}
	if m.state == nil {
		return &core.SchedulerState{Version: 1, DAGs: make(map[string]core.DAGWatermark)}, nil
	}
	return m.state, nil
}

func (m *mockWatermarkStore) Save(_ context.Context, state *core.SchedulerState) error {
	if m.saveErr != nil {
		return m.saveErr
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.saved = append(m.saved, state)
	return nil
}

func (m *mockWatermarkStore) lastSaved() *core.SchedulerState {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.saved) == 0 {
		return nil
	}
	return m.saved[len(m.saved)-1]
}

func newTestCatchupManager(store core.WatermarkStore) *CatchupManager {
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

	parsed, err := cron.ParseStandard("0 * * * *") // hourly
	require.NoError(t, err)

	store := &mockWatermarkStore{
		state: &core.SchedulerState{
			Version:  1,
			LastTick: time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC),
			DAGs:     make(map[string]core.DAGWatermark),
		},
	}
	cm := newTestCatchupManager(store)

	dags := []*core.DAG{{
		Name:          "test-dag",
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{{Expression: "0 * * * *", Parsed: parsed}},
	}}

	err = cm.Init(context.Background(), dags)
	require.NoError(t, err)

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

	parsed, err := cron.ParseStandard("0 * * * *")
	require.NoError(t, err)

	store := &mockWatermarkStore{
		state: &core.SchedulerState{
			Version:  1,
			LastTick: time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC),
			DAGs:     make(map[string]core.DAGWatermark),
		},
	}
	cm := newTestCatchupManager(store)

	dag := &core.DAG{
		Name:          "test-dag",
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{{Expression: "0 * * * *", Parsed: parsed}},
	}
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
	routed = cm.RouteToBuffer("other-dag", QueueItem{})
	assert.False(t, routed)
}

func TestCatchupManager_ProcessBuffersDispatches(t *testing.T) {
	t.Parallel()

	parsed, err := cron.ParseStandard("0 * * * *")
	require.NoError(t, err)

	var dispatched []string
	store := &mockWatermarkStore{
		state: &core.SchedulerState{
			Version:  1,
			LastTick: time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC),
			DAGs:     make(map[string]core.DAGWatermark),
		},
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

	dag := &core.DAG{
		Name:          "my-dag",
		CatchupWindow: 6 * time.Hour,
		Schedule:      []core.Schedule{{Expression: "0 * * * *", Parsed: parsed}},
	}
	require.NoError(t, cm.Init(context.Background(), []*core.DAG{dag}))

	// ProcessBuffers drains one per tick
	cm.ProcessBuffers(context.Background())
	assert.Len(t, dispatched, 1)
	assert.Equal(t, "my-dag:run-1", dispatched[0])
}

func TestCatchupManager_ProcessBuffersSkipOverlap(t *testing.T) {
	t.Parallel()

	parsed, err := cron.ParseStandard("0 * * * *")
	require.NoError(t, err)

	store := &mockWatermarkStore{
		state: &core.SchedulerState{
			Version:  1,
			LastTick: time.Date(2026, 2, 7, 11, 0, 0, 0, time.UTC),
			DAGs:     make(map[string]core.DAGWatermark),
		},
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

	dag := &core.DAG{
		Name:          "skip-dag",
		CatchupWindow: 6 * time.Hour,
		OverlapPolicy: core.OverlapPolicySkip,
		Schedule:      []core.Schedule{{Expression: "0 * * * *", Parsed: parsed}},
	}
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
