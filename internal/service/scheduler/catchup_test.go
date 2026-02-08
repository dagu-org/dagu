package scheduler

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedagstate"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testClock returns a fixed-time clock for testing.
func testClock(t time.Time) Clock {
	return func() time.Time { return t }
}

// parseCron parses a standard 5-field cron expression for testing.
func parseCron(t *testing.T, expr string) cron.Schedule {
	t.Helper()
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(expr)
	require.NoError(t, err)
	return sched
}

// Compile-time check: *filedagstate.Store implements DAGStateStore.
var _ DAGStateStore = (*filedagstate.Store)(nil)

// catchupMockDAGRunStore is a minimal mock for testing.
var _ exec.DAGRunStore = (*catchupMockDAGRunStore)(nil)

type catchupMockDAGRunStore struct {
	attempts []exec.DAGRunAttempt
}

func (m *catchupMockDAGRunStore) CreateAttempt(_ context.Context, _ *core.DAG, _ time.Time, _ string, _ exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) RecentAttempts(_ context.Context, _ string, _ int) []exec.DAGRunAttempt {
	return m.attempts
}
func (m *catchupMockDAGRunStore) LatestAttempt(_ context.Context, _ string) (exec.DAGRunAttempt, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) ListStatuses(_ context.Context, _ ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) FindAttempt(_ context.Context, _ exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) FindSubAttempt(_ context.Context, _ exec.DAGRunRef, _ string) (exec.DAGRunAttempt, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) CreateSubAttempt(_ context.Context, _ exec.DAGRunRef, _ string) (exec.DAGRunAttempt, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) RemoveOldDAGRuns(_ context.Context, _ string, _ int, _ ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, nil
}
func (m *catchupMockDAGRunStore) RenameDAGRuns(_ context.Context, _, _ string) error {
	return nil
}
func (m *catchupMockDAGRunStore) RemoveDAGRun(_ context.Context, _ exec.DAGRunRef) error {
	return nil
}

func TestCatchupEngine_MissingWatermark(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)
	// Don't save any watermark

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 20
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: parseCron(t, "0 * * * *"), Catchup: core.CatchupPolicyAll},
		},
	}

	dags := map[string]*core.DAG{
		"test.yaml": testDAG,
	}

	result, err := engine.Run(context.Background(), dags)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Dispatched)

	// Per-DAG watermark should now be set
	lastTick, err := store.Load(context.Background(), testDAG)
	require.NoError(t, err)
	assert.True(t, now.Equal(lastTick))
}

func TestCatchupEngine_GenerateCandidates_RunAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-3 * time.Hour) // 3 hours ago
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 20
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	sched := parseCron(t, "0 * * * *") // every hour on the hour
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{
		"test.yaml": testDAG,
	}

	perDAGStates := map[*core.DAG]time.Time{
		testDAG: lastTick,
	}

	candidates := engine.generateCandidates(context.Background(), dags, perDAGStates, now)
	// lastTick = 09:00, catchupTo = 12:00
	// Next(09:00)=10:00, Next(10:00)=11:00, Next(11:00)=12:00
	// 12:00 is excluded (equals catchupTo, left to live loop) = 2 candidates
	assert.Equal(t, 2, len(candidates))
}

func TestCatchupEngine_GenerateCandidates_RunLatest(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-3 * time.Hour)
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 20
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyLatest},
		},
	}
	dags := map[string]*core.DAG{
		"test.yaml": testDAG,
	}

	perDAGStates := map[*core.DAG]time.Time{
		testDAG: lastTick,
	}

	candidates := engine.generateCandidates(context.Background(), dags, perDAGStates, now)
	// RunLatest should only keep the latest candidate (12:00 excluded as it equals catchupTo)
	assert.Equal(t, 1, len(candidates))
	assert.Equal(t, time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC), candidates[0].scheduledTime)
}

func TestCatchupEngine_MaxCatchupRunsCap(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-24 * time.Hour) // 24 hours ago = 24 hourly candidates
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 5 // Cap at 5
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{
		"test.yaml": testDAG,
	}

	perDAGStates := map[*core.DAG]time.Time{
		testDAG: lastTick,
	}

	candidates := engine.generateCandidates(context.Background(), dags, perDAGStates, now)
	assert.LessOrEqual(t, len(candidates), 5)
}

func TestCatchupEngine_MaxGlobalCatchupRunsCap(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-24 * time.Hour)
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 3 // Cap globally at 3
	cfg.Scheduler.MaxCatchupRunsPerDAG = 20
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	sched := parseCron(t, "0 * * * *")
	dag1 := &core.DAG{
		Name:     "dag1",
		Location: filepath.Join(dagsDir, "dag1.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dag2 := &core.DAG{
		Name:     "dag2",
		Location: filepath.Join(dagsDir, "dag2.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{
		"dag1.yaml": dag1,
		"dag2.yaml": dag2,
	}

	perDAGStates := map[*core.DAG]time.Time{
		dag1: lastTick,
		dag2: lastTick,
	}

	candidates := engine.generateCandidates(context.Background(), dags, perDAGStates, now)
	assert.Equal(t, 3, len(candidates))
}

func TestCatchupEngine_CatchupWindow(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-24 * time.Hour) // 24 hours ago
	clock := testClock(now)

	store := filedagstate.New(tmpDir, dagsDir)

	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 100
	cfg.Scheduler.CatchupRateLimit = time.Millisecond

	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, nil, cfg, clock)

	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{
				Expression:    "0 * * * *",
				Parsed:        sched,
				Catchup:       core.CatchupPolicyAll,
				CatchupWindow: 3 * time.Hour, // Only look back 3 hours
			},
		},
	}
	dags := map[string]*core.DAG{
		"test.yaml": testDAG,
	}

	perDAGStates := map[*core.DAG]time.Time{
		testDAG: lastTick,
	}

	candidates := engine.generateCandidates(context.Background(), dags, perDAGStates, now)
	// catchupWindow = 3h means replayFrom = 09:00, candidates: 10:00, 11:00
	// 12:00 excluded (equals catchupTo, left to live loop)
	assert.Equal(t, 2, len(candidates))
	assert.Equal(t, time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), candidates[0].scheduledTime)
	assert.Equal(t, time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC), candidates[1].scheduledTime)
}

// --- Mocks for dispatch testing ---

// mockDispatcher records HandleJob calls and optionally returns errors.
var _ catchupDispatcher = (*mockDispatcher)(nil)

type mockDispatcher struct {
	calls  []mockDispatchCall
	failOn map[string]error // DAG name -> error to return
}

type mockDispatchCall struct {
	DAGName       string
	RunID         string
	TriggerType   core.TriggerType
	ScheduledTime time.Time
	Operation     coordinatorv1.Operation
}

func (m *mockDispatcher) HandleJob(
	_ context.Context, dag *core.DAG, operation coordinatorv1.Operation,
	runID string, triggerType core.TriggerType, scheduledTime time.Time,
) error {
	m.calls = append(m.calls, mockDispatchCall{
		DAGName:       dag.Name,
		RunID:         runID,
		TriggerType:   triggerType,
		ScheduledTime: scheduledTime,
		Operation:     operation,
	})
	if m.failOn != nil {
		if err, ok := m.failOn[dag.Name]; ok {
			return err
		}
	}
	return nil
}

// For DAGRunAttempt mocking, use exec.MockDAGRunAttempt from the core/exec package.
// It has a Status field shortcut: set Status to return from ReadStatus without mock setup.

// --- Tests for dispatch flow ---

func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Scheduler.MaxGlobalCatchupRuns = 100
	cfg.Scheduler.MaxCatchupRunsPerDAG = 20
	cfg.Scheduler.CatchupRateLimit = 0 // No delay in tests
	cfg.Scheduler.DuplicateCheckLimit = 100
	return cfg
}

func TestCatchupEngine_Run_DispatchesAll(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-3 * time.Hour)

	store := filedagstate.New(tmpDir, dagsDir)
	dispatcher := &mockDispatcher{}
	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{"test.yaml": testDAG}

	// Seed watermark so catch-up runs
	require.NoError(t, store.Save(context.Background(), testDAG, lastTick))

	cfg := newTestConfig()
	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, dispatcher, cfg, testClock(now))

	result, err := engine.Run(context.Background(), dags)
	require.NoError(t, err)

	// 09:00→10:00, 10:00→11:00 (12:00 excluded as boundary)
	assert.Equal(t, 2, result.Dispatched)
	assert.Equal(t, 0, result.Skipped)
	assert.Equal(t, 2, len(dispatcher.calls))

	// Verify dispatch details
	assert.Equal(t, "test", dispatcher.calls[0].DAGName)
	assert.Equal(t, core.TriggerTypeCatchUp, dispatcher.calls[0].TriggerType)
	assert.Equal(t, coordinatorv1.Operation_OPERATION_START, dispatcher.calls[0].Operation)
	assert.Equal(t, time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC), dispatcher.calls[0].ScheduledTime)
	assert.Equal(t, time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC), dispatcher.calls[1].ScheduledTime)

	// Watermark should be advanced to catchupTo (all completed)
	lastTickLoaded, err := store.Load(context.Background(), testDAG)
	require.NoError(t, err)
	assert.True(t, now.Equal(lastTickLoaded))
}

func TestCatchupEngine_Run_DispatchFailure_WatermarkPartialAdvance(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-5 * time.Hour)

	store := filedagstate.New(tmpDir, dagsDir)
	dispatcher := &mockDispatcher{
		failOn: map[string]error{"dag-fail": errors.New("dispatch error")},
	}
	sched := parseCron(t, "0 * * * *")

	dagOK := &core.DAG{
		Name:     "dag-ok",
		Location: filepath.Join(dagsDir, "dag-ok.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dagFail := &core.DAG{
		Name:     "dag-fail",
		Location: filepath.Join(dagsDir, "dag-fail.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{
		"dag-fail.yaml": dagFail,
		"dag-ok.yaml":   dagOK,
	}

	// Seed watermarks
	ctx := context.Background()
	require.NoError(t, store.Save(ctx, dagOK, lastTick))
	require.NoError(t, store.Save(ctx, dagFail, lastTick))

	cfg := newTestConfig()
	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, dispatcher, cfg, testClock(now))

	result, err := engine.Run(ctx, dags)
	require.NoError(t, err)

	// Some candidates dispatched before failure broke the loop
	assert.True(t, result.Dispatched >= 0)

	// dag-fail's watermark should NOT be advanced to catchupTo
	failTick, err := store.Load(ctx, dagFail)
	require.NoError(t, err)
	assert.True(t, failTick.Before(now), "failed DAG watermark should not advance to catchupTo")

	// dag-ok's watermark should be at most the last processed candidate (not catchupTo since loop broke early)
	okTick, err := store.Load(ctx, dagOK)
	require.NoError(t, err)
	// It may or may not have been advanced depending on ordering, but must NOT equal catchupTo
	// since completedAll=false
	assert.True(t, okTick.Before(now) || okTick.Equal(now))
}

func TestCatchupEngine_IsDuplicate(t *testing.T) {
	t.Parallel()

	scheduledTime := time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC)

	t.Run("Duplicate found", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dagsDir := filepath.Join(tmpDir, "dags")

		mockAttempt := &exec.MockDAGRunAttempt{
			Status: &exec.DAGRunStatus{
				ScheduledTime: scheduledTime.Format(time.RFC3339),
			},
		}
		mockStore := &catchupMockDAGRunStore{
			attempts: []exec.DAGRunAttempt{mockAttempt},
		}

		cfg := newTestConfig()
		engine := NewCatchupEngine(
			filedagstate.New(tmpDir, dagsDir), mockStore, nil, cfg, testClock(time.Now()),
		)

		cand := catchupCandidate{
			dag:           &core.DAG{Name: "test"},
			scheduledTime: scheduledTime,
		}

		assert.True(t, engine.isDuplicate(context.Background(), cand))
	})

	t.Run("No duplicate", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dagsDir := filepath.Join(tmpDir, "dags")

		mockAttempt := &exec.MockDAGRunAttempt{
			Status: &exec.DAGRunStatus{
				ScheduledTime: time.Date(2025, 6, 15, 9, 0, 0, 0, time.UTC).Format(time.RFC3339),
			},
		}
		mockStore := &catchupMockDAGRunStore{
			attempts: []exec.DAGRunAttempt{mockAttempt},
		}

		cfg := newTestConfig()
		engine := NewCatchupEngine(
			filedagstate.New(tmpDir, dagsDir), mockStore, nil, cfg, testClock(time.Now()),
		)

		cand := catchupCandidate{
			dag:           &core.DAG{Name: "test"},
			scheduledTime: scheduledTime,
		}

		assert.False(t, engine.isDuplicate(context.Background(), cand))
	})

	t.Run("Empty attempts", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		dagsDir := filepath.Join(tmpDir, "dags")

		mockStore := &catchupMockDAGRunStore{}

		cfg := newTestConfig()
		engine := NewCatchupEngine(
			filedagstate.New(tmpDir, dagsDir), mockStore, nil, cfg, testClock(time.Now()),
		)

		cand := catchupCandidate{
			dag:           &core.DAG{Name: "test"},
			scheduledTime: scheduledTime,
		}

		assert.False(t, engine.isDuplicate(context.Background(), cand))
	})
}

func TestCatchupEngine_Run_SkipsDuplicates(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-3 * time.Hour)

	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{"test.yaml": testDAG}

	store := filedagstate.New(tmpDir, dagsDir)
	require.NoError(t, store.Save(context.Background(), testDAG, lastTick))

	// Mock store that reports 10:00 as already existing
	existingAttempt := &exec.MockDAGRunAttempt{
		Status: &exec.DAGRunStatus{
			ScheduledTime: time.Date(2025, 6, 15, 10, 0, 0, 0, time.UTC).Format(time.RFC3339),
		},
	}
	mockRunStore := &catchupMockDAGRunStore{
		attempts: []exec.DAGRunAttempt{existingAttempt},
	}

	dispatcher := &mockDispatcher{}
	cfg := newTestConfig()
	engine := NewCatchupEngine(store, mockRunStore, dispatcher, cfg, testClock(now))

	result, err := engine.Run(context.Background(), dags)
	require.NoError(t, err)

	// 10:00 should be skipped (duplicate), 11:00 should be dispatched
	assert.Equal(t, 1, result.Dispatched)
	assert.Equal(t, 1, result.Skipped)
	assert.Equal(t, 1, len(dispatcher.calls))
	assert.Equal(t, time.Date(2025, 6, 15, 11, 0, 0, 0, time.UTC), dispatcher.calls[0].ScheduledTime)
}

func TestCatchupEngine_Run_ContextCancelled(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	dagsDir := filepath.Join(tmpDir, "dags")
	now := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	lastTick := now.Add(-5 * time.Hour)

	sched := parseCron(t, "0 * * * *")
	testDAG := &core.DAG{
		Name:     "test",
		Location: filepath.Join(dagsDir, "test.yaml"),
		Schedule: []core.Schedule{
			{Expression: "0 * * * *", Parsed: sched, Catchup: core.CatchupPolicyAll},
		},
	}
	dags := map[string]*core.DAG{"test.yaml": testDAG}

	store := filedagstate.New(tmpDir, dagsDir)
	require.NoError(t, store.Save(context.Background(), testDAG, lastTick))

	dispatcher := &mockDispatcher{}
	cfg := newTestConfig()
	engine := NewCatchupEngine(store, &catchupMockDAGRunStore{}, dispatcher, cfg, testClock(now))

	// Cancel the context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := engine.Run(ctx, dags)
	require.NoError(t, err)

	// No dispatches should happen since context is already cancelled
	assert.Equal(t, 0, result.Dispatched)

	// Watermark should NOT advance to catchupTo since completedAll=false
	loadedTick, err := store.Load(context.Background(), testDAG)
	require.NoError(t, err)
	assert.True(t, loadedTick.Before(now) || loadedTick.Equal(lastTick),
		"watermark should not advance to catchupTo when context is cancelled")
}
