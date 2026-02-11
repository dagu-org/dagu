package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

// DAGChangeType identifies the kind of DAG lifecycle event.
type DAGChangeType int

const (
	DAGChangeAdded DAGChangeType = iota
	DAGChangeUpdated
	DAGChangeDeleted
)

// DAGChangeEvent represents a DAG lifecycle event emitted by the EntryReader.
type DAGChangeEvent struct {
	Type    DAGChangeType
	DAG     *core.DAG // non-nil for Added/Updated
	DAGName string    // always set (needed for delete)
}

// PlannedRun represents a run that the TickPlanner has decided should be dispatched.
type PlannedRun struct {
	DAG           *core.DAG
	RunID         string
	ScheduledTime time.Time
	TriggerType   core.TriggerType
	ScheduleType  ScheduleType
}

// DispatchFunc dispatches a catch-up or scheduled run for the given DAG.
type DispatchFunc func(ctx context.Context, dag *core.DAG, runID string, triggerType core.TriggerType) error

// RunIDFunc generates a unique run ID.
type RunIDFunc func(ctx context.Context) (string, error)

// IsRunningFunc checks if a DAG has any active run.
type IsRunningFunc func(ctx context.Context, dag *core.DAG) (bool, error)

// GetLatestStatusFunc retrieves the latest status of a DAG.
type GetLatestStatusFunc func(ctx context.Context, dag *core.DAG) (exec.DAGRunStatus, error)

// IsSuspendedFunc checks whether a DAG is currently suspended.
type IsSuspendedFunc func(ctx context.Context, dagName string) bool

// StopFunc stops a running DAG.
type StopFunc func(ctx context.Context, dag *core.DAG) error

// RestartFunc restarts a DAG unconditionally.
type RestartFunc func(ctx context.Context, dag *core.DAG) error

// TickPlannerConfig holds the dependencies for creating a TickPlanner.
type TickPlannerConfig struct {
	WatermarkStore  WatermarkStore
	IsSuspended     IsSuspendedFunc
	GetLatestStatus GetLatestStatusFunc
	IsRunning       IsRunningFunc
	GenRunID        RunIDFunc
	Dispatch        DispatchFunc
	Stop            StopFunc
	Restart         RestartFunc
	Clock           Clock
	Location        *time.Location // timezone for cron schedule evaluation
	Events          <-chan DAGChangeEvent
}

// TickPlanner is the unified scheduling decision module.
// Given the current time, it determines which start-schedule runs should dispatch,
// tracks progress via watermarks, and reacts to DAG lifecycle changes.
//
// Thread safety:
//   - entries and buffers are protected by entryMu (accessed from drainEvents
//     goroutine and cronLoop's Plan).
//   - watermarkState is shared with the flusher goroutine and protected by mu.
//   - Plan() holds entryMu during I/O calls (IsSuspended, IsRunning,
//     GetLatestStatus, GenRunID). This is intentional: the lock prevents
//     event processing during planning, ensuring a consistent snapshot of
//     entries for the entire plan cycle.
//   - lastPlanResult is accessed only from cronLoop (Plan writes, Advance reads)
//     and requires no lock. See field comment for details.
type TickPlanner struct {
	cfg TickPlannerConfig

	// watermark state (protected by mu)
	mu             sync.RWMutex
	watermarkState *SchedulerState
	watermarkDirty atomic.Bool

	// per-DAG tracking (protected by entryMu)
	entryMu sync.Mutex
	entries map[string]*plannerEntry
	buffers map[string]*ScheduleBuffer

	// lastPlanResult holds the runs from the most recent Plan() call.
	// It is written by Plan() and read by Advance(). Both are called
	// sequentially from the same goroutine (cronLoop in scheduler.go),
	// so no lock is needed. Do NOT call Plan() or Advance() from
	// different goroutines without external synchronization.
	lastPlanResult []PlannedRun

	// lifecycle
	started atomic.Bool
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// plannerEntry tracks a single DAG's scheduling metadata.
type plannerEntry struct {
	dag *core.DAG
}

// NewTickPlanner creates a new TickPlanner with the given configuration.
// Nil config fields are replaced with no-op defaults.
func NewTickPlanner(cfg TickPlannerConfig) *TickPlanner {
	if cfg.WatermarkStore == nil {
		cfg.WatermarkStore = noopWatermarkStore{}
	}
	if cfg.IsSuspended == nil {
		cfg.IsSuspended = func(context.Context, string) bool { return false }
	}
	if cfg.IsRunning == nil {
		cfg.IsRunning = func(context.Context, *core.DAG) (bool, error) { return false, nil }
	}
	if cfg.GetLatestStatus == nil {
		cfg.GetLatestStatus = func(context.Context, *core.DAG) (exec.DAGRunStatus, error) {
			return exec.DAGRunStatus{}, nil
		}
	}
	if cfg.Stop == nil {
		cfg.Stop = func(context.Context, *core.DAG) error { return nil }
	}
	if cfg.Restart == nil {
		cfg.Restart = func(context.Context, *core.DAG) error { return nil }
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.Location == nil {
		cfg.Location = time.Local
	}
	if cfg.GenRunID == nil {
		cfg.GenRunID = func(context.Context) (string, error) {
			return "", fmt.Errorf("genRunID not configured")
		}
	}
	if cfg.Dispatch == nil {
		cfg.Dispatch = func(context.Context, *core.DAG, string, core.TriggerType) error {
			return fmt.Errorf("dispatch not configured")
		}
	}
	return &TickPlanner{
		cfg:     cfg,
		entries: make(map[string]*plannerEntry),
		buffers: make(map[string]*ScheduleBuffer),
	}
}

// Init loads watermark state and computes catchup buffers for existing DAGs.
func (tp *TickPlanner) Init(ctx context.Context, dags []*core.DAG) error {
	tp.entryMu.Lock()
	defer tp.entryMu.Unlock()

	// Populate entries from existing DAGs
	for _, dag := range dags {
		tp.entries[dag.Name] = &plannerEntry{dag: dag}
	}

	state, err := tp.cfg.WatermarkStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load watermark state", tag.Error(err))
		state = &SchedulerState{Version: 1, DAGs: make(map[string]DAGWatermark)}
	}
	if state.DAGs == nil {
		state.DAGs = make(map[string]DAGWatermark)
	}

	// Prune stale DAG entries that no longer exist on disk.
	activeDags := make(map[string]struct{}, len(dags))
	for _, dag := range dags {
		activeDags[dag.Name] = struct{}{}
	}
	pruned := 0
	for name := range state.DAGs {
		if _, ok := activeDags[name]; !ok {
			delete(state.DAGs, name)
			pruned++
		}
	}
	if pruned > 0 {
		logger.Info(ctx, "Pruned stale watermark entries",
			slog.Int("pruned", pruned),
		)
	}

	tp.mu.Lock()
	tp.watermarkState = state
	tp.mu.Unlock()

	logger.Info(ctx, "Loaded scheduler watermark",
		slog.Time("lastTick", state.LastTick),
		slog.Int("dagCount", len(state.DAGs)),
	)

	tp.initBuffers(ctx, dags)
	return nil
}

// initBuffers creates per-DAG queues for DAGs with CatchupWindow > 0
// and enqueues catch-up items.
func (tp *TickPlanner) initBuffers(ctx context.Context, dags []*core.DAG) {
	// Snapshot watermark state under the lock. Although initBuffers is only
	// called from Init (before Start), we snapshot defensively to avoid
	// reading the shared DAGs map outside the lock.
	tp.mu.RLock()
	lastTick := tp.watermarkState.LastTick
	dagWatermarks := make(map[string]DAGWatermark, len(tp.watermarkState.DAGs))
	maps.Copy(dagWatermarks, tp.watermarkState.DAGs)
	tp.mu.RUnlock()

	now := tp.cfg.Clock()
	var totalMissed int

	for _, dag := range dags {
		if dag.CatchupWindow <= 0 {
			continue
		}

		var lastScheduledTime time.Time
		if wm, ok := dagWatermarks[dag.Name]; ok {
			lastScheduledTime = wm.LastScheduledTime
		}

		replayFrom := ComputeReplayFrom(dag.CatchupWindow, lastTick, lastScheduledTime, now)
		missed := ComputeMissedIntervals(dag.Schedule, replayFrom, now)

		if len(missed) == 0 {
			continue
		}

		totalMissed += len(missed)

		logger.Info(ctx, "Catch-up planned",
			tag.DAG(dag.Name),
			slog.Int("missedCount", len(missed)),
			slog.Time("replayFrom", replayFrom),
			slog.Time("replayTo", now),
		)

		q := NewScheduleBuffer(dag.Name, dag.OverlapPolicy)
		tp.buffers[dag.Name] = q

		for _, t := range missed {
			if !q.Send(QueueItem{
				DAG:           dag,
				ScheduledTime: t,
				TriggerType:   core.TriggerTypeCatchUp,
				ScheduleType:  ScheduleTypeStart,
			}) {
				logger.Error(ctx, "Catch-up buffer full, dropping remaining items",
					tag.DAG(dag.Name),
					slog.Int("buffered", q.Len()),
					slog.Int("dropped", len(missed)-q.Len()),
				)
				break
			}
		}

		if dag.OverlapPolicy == core.OverlapPolicyLatest && q.Len() > 1 {
			dropped := q.DropAllButLast()
			totalMissed -= len(dropped)
			tp.advanceDAGWatermark(dag.Name, dropped[len(dropped)-1].ScheduledTime)
		}
	}

	if totalMissed > 0 {
		logger.Info(ctx, "Catch-up initialization complete",
			slog.Int("dagCount", len(tp.buffers)),
			slog.Int("totalMissedRuns", totalMissed),
		)
	}
}

// Plan drains queued DAG events, then returns ordered runs to dispatch this tick.
// Includes live scheduled runs and catchup runs. Only returns runs that pass
// all guards (not running, not suspended, not finished, not skipped).
// The caller just dispatches.
func (tp *TickPlanner) Plan(ctx context.Context, now time.Time) []PlannedRun {
	tp.entryMu.Lock()
	defer tp.entryMu.Unlock()

	var candidates []PlannedRun

	for dagName, entry := range tp.entries {
		// Check suspension.
		// IsSuspended is keyed by filename stem (not dag.Name), matching the
		// file-based suspension flag system in filedag/store.go.
		dagBaseName := strings.TrimSuffix(filepath.Base(entry.dag.Location), filepath.Ext(entry.dag.Location))
		if tp.cfg.IsSuspended(ctx, dagBaseName) {
			continue
		}

		// Check catchup buffer first (catchup has priority over live)
		catchupProduced := false
		if buf, ok := tp.buffers[dagName]; ok {
			item, hasItem := buf.Peek()
			if !hasItem {
				delete(tp.buffers, dagName)
			} else {
				running, err := tp.cfg.IsRunning(ctx, item.DAG)
				if err != nil {
					logger.Error(ctx, "Failed to check if DAG is running, assuming not running",
						tag.DAG(dagName),
						tag.Error(err),
					)
					running = false
				}

				if !running {
					// For "latest", collapse to most recent before popping.
					if buf.overlapPolicy == core.OverlapPolicyLatest && buf.Len() > 1 {
						dropped := buf.DropAllButLast()
						tp.advanceDAGWatermark(dagName, dropped[len(dropped)-1].ScheduledTime)
						// Re-peek: front changed from oldest to latest
						item, _ = buf.Peek()
					}
					buf.Pop()
					run, ok := tp.createPlannedRun(ctx, item.DAG, item.ScheduledTime, item.TriggerType)
					if ok {
						candidates = append(candidates, run)
						catchupProduced = true
					}
				} else {
					switch buf.overlapPolicy {
					case core.OverlapPolicySkip:
						popped, _ := buf.Pop()
						logger.Info(ctx, "Catch-up run skipped (overlap policy: skip)",
							tag.DAG(dagName),
						)
						tp.advanceDAGWatermark(dagName, popped.ScheduledTime)
					case core.OverlapPolicyAll:
						// leave in buffer, retry next tick
					case core.OverlapPolicyLatest:
						// Collapse to latest, advance watermark past discarded items.
						dropped := buf.DropAllButLast()
						if len(dropped) > 0 {
							tp.advanceDAGWatermark(dagName, dropped[len(dropped)-1].ScheduledTime)
						}
						// Leave the single remaining (latest) item for retry next tick.
					default:
						popped, _ := buf.Pop()
						logger.Warn(ctx, "Unknown overlap policy, treating as skip",
							tag.DAG(dagName),
							slog.String("overlapPolicy", string(buf.overlapPolicy)),
						)
						tp.advanceDAGWatermark(dagName, popped.ScheduledTime)
					}
				}

				// Clean up empty buffers
				if buf.Len() == 0 {
					delete(tp.buffers, dagName)
				}
			}
		}

		// If catchup produced a run, skip live eval (catchup has priority)
		if catchupProduced {
			continue
		}

		// Evaluate cron schedules for live start runs.
		// Start schedules use raw `now`: the robfig/cron library applies the
		// schedule's timezone internally, so no extra conversion is needed.
		for _, schedule := range entry.dag.Schedule {
			next, due := scheduleDueAt(schedule, now)
			if !due {
				continue
			}
			if !tp.shouldRun(ctx, entry.dag, next, schedule) {
				continue
			}
			run, ok := tp.createPlannedRun(ctx, entry.dag, next, core.TriggerTypeScheduler)
			if ok {
				candidates = append(candidates, run)
			}
		}

		// Evaluate stop schedules.
		// Stop and restart schedules convert `now` to the configured Location so
		// that the cron evaluation matches the wall-clock time the user expects.
		// This preserves parity with the legacy invokeJobs implementation.
		evalTime := now.In(tp.cfg.Location)
		for _, schedule := range entry.dag.StopSchedule {
			next, due := scheduleDueAt(schedule, evalTime)
			if !due {
				continue
			}

			// Guard: DAG must be running before issuing a stop
			latestStatus, err := tp.cfg.GetLatestStatus(ctx, entry.dag)
			if err != nil {
				logger.Error(ctx, "Failed to fetch DAG status for stop schedule",
					tag.DAG(dagName), tag.Error(err))
				continue
			}
			if latestStatus.Status != core.Running {
				continue
			}

			candidates = append(candidates, PlannedRun{
				DAG:           entry.dag,
				ScheduledTime: next,
				ScheduleType:  ScheduleTypeStop,
			})
		}

		// Evaluate restart schedules (no guard -- fires unconditionally).
		for _, schedule := range entry.dag.RestartSchedule {
			next, due := scheduleDueAt(schedule, evalTime)
			if !due {
				continue
			}
			candidates = append(candidates, PlannedRun{
				DAG:           entry.dag,
				ScheduledTime: next,
				ScheduleType:  ScheduleTypeRestart,
			})
		}
	}

	tp.lastPlanResult = candidates
	return candidates
}

// shouldRun checks all guards for a live scheduled run.
func (tp *TickPlanner) shouldRun(ctx context.Context, dag *core.DAG, scheduledTime time.Time, schedule core.Schedule) bool {
	// Guard 1: isRunning (uses process-level check)
	running, err := tp.cfg.IsRunning(ctx, dag)
	if err != nil {
		logger.Error(ctx, "Failed to check if DAG is running",
			tag.DAG(dag.Name),
			tag.Error(err),
		)
		return false
	}
	if running {
		return false
	}

	latestStatus, err := tp.cfg.GetLatestStatus(ctx, dag)
	if err != nil {
		logger.Error(ctx, "Failed to fetch latest DAG status",
			tag.DAG(dag.Name),
			tag.Error(err),
		)
		return false
	}

	// Also check status-based running (belt and suspenders)
	if latestStatus.Status == core.Running {
		return false
	}

	// Guard 2: alreadyFinished — check if last run started at/after scheduled time
	latestStartedAt, err := stringutil.ParseTime(latestStatus.StartedAt)
	if err == nil {
		// Consider queued time as well
		if latestStatus.QueuedAt != "" {
			queuedAt, parseErr := stringutil.ParseTime(latestStatus.QueuedAt)
			if parseErr == nil && queuedAt.Before(latestStartedAt) {
				latestStartedAt = queuedAt
			}
		}
		latestStartedAt = latestStartedAt.Truncate(time.Minute)
		if !latestStartedAt.Before(scheduledTime) {
			return false
		}

		// Guard 3: skipIfSuccessful
		if dag.SkipIfSuccessful && latestStatus.Status == core.Succeeded && schedule.Parsed != nil {
			prevExecTime := computePrevExecTime(scheduledTime, schedule)
			if !latestStartedAt.Before(prevExecTime) && latestStartedAt.Before(scheduledTime) {
				logger.Info(ctx, "Skipping job due to successful prior run",
					tag.DAG(dag.Name),
					slog.String("start-time", latestStartedAt.Format(time.RFC3339)),
				)
				return false
			}
		}
	}

	return true
}

// computePrevExecTime calculates the previous schedule fire time before next.
// It walks forward from (next - 32 days) to find the last fire time before next.
// 32 days covers monthly schedules, the most common sparse cron interval.
// This correctly handles non-uniform cron schedules (e.g., "0 9,17 * * *").
func computePrevExecTime(next time.Time, schedule core.Schedule) time.Time {
	if schedule.Parsed == nil {
		return next
	}
	// Walk forward from 32 days before next to find the last fire time before next.
	seed := next.Add(-32 * 24 * time.Hour)
	var prev time.Time
	t := schedule.Parsed.Next(seed)
	for t.Before(next) {
		prev = t
		t = schedule.Parsed.Next(t)
	}
	if prev.IsZero() {
		// Fallback: no previous fire time found within the 7-day window.
		// Use interval heuristic as last resort.
		nextNext := schedule.Parsed.Next(next.Add(time.Second))
		return next.Add(-(nextNext.Sub(next)))
	}
	return prev
}

// scheduleDueAt returns the next fire time if the schedule is due at the given
// time, or the zero value if the schedule should not fire.
func scheduleDueAt(schedule core.Schedule, now time.Time) (time.Time, bool) {
	if schedule.Parsed == nil {
		return time.Time{}, false
	}
	next := schedule.Parsed.Next(now.Add(-time.Second))
	if next.After(now) {
		return time.Time{}, false
	}
	return next, true
}

// createPlannedRun generates a run ID and constructs a PlannedRun.
func (tp *TickPlanner) createPlannedRun(ctx context.Context, dag *core.DAG, scheduledTime time.Time, triggerType core.TriggerType) (PlannedRun, bool) {
	runID, err := tp.cfg.GenRunID(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to generate run ID",
			tag.DAG(dag.Name),
			tag.Error(err),
		)
		return PlannedRun{}, false
	}

	return PlannedRun{
		DAG:           dag,
		RunID:         runID,
		ScheduledTime: scheduledTime,
		TriggerType:   triggerType,
	}, true
}

// Advance records that this tick was processed. Updates global and per-DAG watermarks.
func (tp *TickPlanner) Advance(now time.Time) {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	tp.watermarkState.LastTick = now

	for _, run := range tp.lastPlanResult {
		if run.ScheduleType != ScheduleTypeStart {
			continue
		}
		if run.TriggerType == core.TriggerTypeCatchUp {
			continue // watermark updated in DispatchRun on success
		}
		tp.watermarkState.DAGs[run.DAG.Name] = DAGWatermark{
			LastScheduledTime: run.ScheduledTime,
		}
	}

	tp.watermarkDirty.Store(true)
	tp.lastPlanResult = nil
}

// advanceDAGWatermark updates the per-DAG watermark to the given time
// and marks the state as dirty. Caller must NOT hold tp.mu.
func (tp *TickPlanner) advanceDAGWatermark(dagName string, scheduledTime time.Time) {
	tp.mu.Lock()
	tp.watermarkState.DAGs[dagName] = DAGWatermark{
		LastScheduledTime: scheduledTime,
	}
	tp.watermarkDirty.Store(true)
	tp.mu.Unlock()
}

// Flush writes the watermark state to disk if dirty.
// Safe for concurrent use.
func (tp *TickPlanner) Flush(ctx context.Context) {
	if !tp.watermarkDirty.CompareAndSwap(true, false) {
		return
	}

	// Snapshot under read lock to avoid holding the lock during I/O.
	tp.mu.RLock()
	snapshot := &SchedulerState{
		Version:  tp.watermarkState.Version,
		LastTick: tp.watermarkState.LastTick,
		DAGs:     make(map[string]DAGWatermark, len(tp.watermarkState.DAGs)),
	}
	maps.Copy(snapshot.DAGs, tp.watermarkState.DAGs)
	tp.mu.RUnlock()

	if err := tp.cfg.WatermarkStore.Save(ctx, snapshot); err != nil {
		logger.Error(ctx, "Failed to flush watermark state", tag.Error(err))
		tp.watermarkDirty.Store(true)
	}
}

// Start launches the internal goroutines (event drainer + watermark flusher).
func (tp *TickPlanner) Start(ctx context.Context) {
	if !tp.started.CompareAndSwap(false, true) {
		return
	}
	ctx, tp.cancel = context.WithCancel(ctx)
	tp.wg.Add(2)
	go func() {
		defer tp.wg.Done()
		tp.drainEvents(ctx)
	}()
	go func() {
		defer tp.wg.Done()
		tp.startFlusher(ctx)
	}()
}

// Stop cancels internal goroutines, waits for them, and performs a final flush.
func (tp *TickPlanner) Stop(ctx context.Context) {
	if tp.cancel != nil {
		tp.cancel()
	}
	tp.wg.Wait()
	tp.Flush(ctx)
}

// startFlusher runs the periodic watermark flusher. Blocks until ctx is done.
func (tp *TickPlanner) startFlusher(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tp.Flush(ctx)
		}
	}
}

// drainEvents continuously processes DAG change events. Blocks until ctx is done.
func (tp *TickPlanner) drainEvents(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-tp.cfg.Events:
			if !ok {
				return
			}
			tp.entryMu.Lock()
			tp.handleEvent(ctx, event)
			tp.entryMu.Unlock()
		}
	}
}

// handleEvent processes a single DAG change event.
func (tp *TickPlanner) handleEvent(ctx context.Context, event DAGChangeEvent) {
	switch event.Type {
	case DAGChangeAdded:
		if event.DAG == nil {
			return
		}
		tp.entries[event.DAGName] = &plannerEntry{dag: event.DAG}
		// Set watermark to now (new DAGs have no catchup)
		tp.advanceDAGWatermark(event.DAGName, tp.cfg.Clock())
		logger.Info(ctx, "Planner: DAG added", tag.DAG(event.DAGName))

	case DAGChangeUpdated:
		if event.DAG == nil {
			return
		}
		tp.entries[event.DAGName] = &plannerEntry{dag: event.DAG}
		// Remove existing buffer and recompute if catchupWindow > 0
		delete(tp.buffers, event.DAGName)
		if event.DAG.CatchupWindow > 0 {
			tp.recomputeBuffer(ctx, event.DAG)
		}
		logger.Info(ctx, "Planner: DAG updated", tag.DAG(event.DAGName))

	case DAGChangeDeleted:
		delete(tp.entries, event.DAGName)
		delete(tp.buffers, event.DAGName)
		tp.mu.Lock()
		delete(tp.watermarkState.DAGs, event.DAGName)
		tp.watermarkDirty.Store(true)
		tp.mu.Unlock()
		logger.Info(ctx, "Planner: DAG deleted", tag.DAG(event.DAGName))
	}
}

// recomputeBuffer creates a new catch-up buffer for a DAG using the existing watermark.
func (tp *TickPlanner) recomputeBuffer(ctx context.Context, dag *core.DAG) {
	// Snapshot needed values under the lock to avoid reading the shared map
	// after releasing it (Advance and handleEvent can modify DAGs concurrently).
	tp.mu.RLock()
	lastTick := tp.watermarkState.LastTick
	var lastScheduledTime time.Time
	if wm, ok := tp.watermarkState.DAGs[dag.Name]; ok {
		lastScheduledTime = wm.LastScheduledTime
	}
	tp.mu.RUnlock()

	now := tp.cfg.Clock()

	replayFrom := ComputeReplayFrom(dag.CatchupWindow, lastTick, lastScheduledTime, now)
	missed := ComputeMissedIntervals(dag.Schedule, replayFrom, now)

	if len(missed) == 0 {
		return
	}

	q := NewScheduleBuffer(dag.Name, dag.OverlapPolicy)
	for _, t := range missed {
		if !q.Send(QueueItem{
			DAG:           dag,
			ScheduledTime: t,
			TriggerType:   core.TriggerTypeCatchUp,
			ScheduleType:  ScheduleTypeStart,
		}) {
			break
		}
	}

	if dag.OverlapPolicy == core.OverlapPolicyLatest && q.Len() > 1 {
		dropped := q.DropAllButLast()
		tp.advanceDAGWatermark(dag.Name, dropped[len(dropped)-1].ScheduledTime)
	}

	tp.buffers[dag.Name] = q

	logger.Info(ctx, "Recomputed catch-up buffer",
		tag.DAG(dag.Name),
		slog.Int("missedCount", len(missed)),
	)
}

// DispatchRun dispatches a PlannedRun using the configured dispatch functions.
func (tp *TickPlanner) DispatchRun(ctx context.Context, run PlannedRun) {
	logger.Info(ctx, "Dispatching planned run",
		tag.DAG(run.DAG.Name),
		slog.String("scheduleType", run.ScheduleType.String()),
		slog.String("scheduledTime", run.ScheduledTime.Format(time.RFC3339)),
	)

	var err error
	switch run.ScheduleType {
	case ScheduleTypeStart:
		err = tp.cfg.Dispatch(ctx, run.DAG, run.RunID, run.TriggerType)
	case ScheduleTypeStop:
		err = tp.cfg.Stop(ctx, run.DAG)
	case ScheduleTypeRestart:
		err = tp.cfg.Restart(ctx, run.DAG)
	}

	if err != nil {
		logger.Error(ctx, "Failed to dispatch run",
			tag.DAG(run.DAG.Name),
			slog.String("scheduleType", run.ScheduleType.String()),
			tag.Error(err),
		)
		return
	}

	// On successful catchup dispatch, advance the per-DAG watermark.
	if run.TriggerType == core.TriggerTypeCatchUp && run.ScheduleType == ScheduleTypeStart {
		tp.advanceDAGWatermark(run.DAG.Name, run.ScheduledTime)
	}
}
