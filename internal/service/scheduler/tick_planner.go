package scheduler

import (
	"context"
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
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// DAGChangeType identifies the kind of DAG lifecycle event.
type DAGChangeType int

const (
	DAGChangeAdded   DAGChangeType = iota
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

// TickPlannerConfig holds the dependencies for creating a TickPlanner.
type TickPlannerConfig struct {
	WatermarkStore  WatermarkStore
	IsSuspended     IsSuspendedFunc
	GetLatestStatus GetLatestStatusFunc
	IsRunning       IsRunningFunc
	GenRunID        RunIDFunc
	Dispatch        DispatchFunc
	Clock           Clock
	Events          <-chan DAGChangeEvent
}

// TickPlanner is the unified scheduling decision module.
// Given the current time, it determines which start-schedule runs should dispatch,
// tracks progress via watermarks, and reacts to DAG lifecycle changes.
//
// Thread safety:
//   - entries and buffers are only accessed from Plan() and drainEvents(),
//     both called from the cronLoop goroutine. No lock needed.
//   - watermarkState is shared with the flusher goroutine and protected by mu.
type TickPlanner struct {
	cfg TickPlannerConfig

	// watermark state (protected by mu)
	mu             sync.RWMutex
	watermarkState *SchedulerState
	watermarkDirty atomic.Bool

	// per-DAG tracking (accessed only from cronLoop goroutine, no lock needed)
	entries map[string]*plannerEntry
	buffers map[string]*ScheduleBuffer

	// last plan result (for Advance to update watermarks)
	lastPlanResult []PlannedRun
}

// plannerEntry tracks a single DAG's scheduling metadata.
type plannerEntry struct {
	dag       *core.DAG
	schedules []core.Schedule
}

// NewTickPlanner creates a new TickPlanner with the given configuration.
func NewTickPlanner(cfg TickPlannerConfig) *TickPlanner {
	return &TickPlanner{
		cfg:     cfg,
		entries: make(map[string]*plannerEntry),
		buffers: make(map[string]*ScheduleBuffer),
	}
}

// Init loads watermark state and computes catchup buffers for existing DAGs.
func (tp *TickPlanner) Init(ctx context.Context, dags []*core.DAG) error {
	// Populate entries from existing DAGs
	for _, dag := range dags {
		tp.entries[dag.Name] = &plannerEntry{
			dag:       dag,
			schedules: dag.Schedule,
		}
	}

	if tp.cfg.WatermarkStore == nil {
		return nil
	}

	state, err := tp.cfg.WatermarkStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load watermark state", tag.Error(err))
		return nil
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
	tp.mu.RLock()
	state := tp.watermarkState
	tp.mu.RUnlock()

	if state == nil {
		return
	}

	now := tp.cfg.Clock()
	var totalMissed int

	for _, dag := range dags {
		if dag.CatchupWindow <= 0 {
			continue
		}

		lastTick := state.LastTick
		var lastScheduledTime time.Time
		if wm, ok := state.DAGs[dag.Name]; ok {
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
	tp.drainEvents(ctx)

	var candidates []PlannedRun

	for dagName, entry := range tp.entries {
		// Check suspension
		if tp.cfg.IsSuspended != nil {
			dagBaseName := strings.TrimSuffix(filepath.Base(entry.dag.Location), filepath.Ext(entry.dag.Location))
			if tp.cfg.IsSuspended(ctx, dagBaseName) {
				continue
			}
		}

		// Check catchup buffer first (catchup has priority over live)
		catchupProduced := false
		if buf, ok := tp.buffers[dagName]; ok {
			item, hasItem := buf.Peek()
			if !hasItem {
				delete(tp.buffers, dagName)
			} else {
				var running bool
				if tp.cfg.IsRunning != nil {
					var err error
					running, err = tp.cfg.IsRunning(ctx, item.DAG)
					if err != nil {
						logger.Error(ctx, "Failed to check if DAG is running, assuming not running",
							tag.DAG(dagName),
							tag.Error(err),
						)
						running = false
					}
				}

				if !running {
					buf.Pop()
					run, ok := tp.createPlannedRun(ctx, item.DAG, item.ScheduledTime, item.TriggerType)
					if ok {
						candidates = append(candidates, run)
						catchupProduced = true
					}
				} else {
					switch buf.overlapPolicy {
					case core.OverlapPolicySkip:
						buf.Pop()
						logger.Info(ctx, "Catch-up run skipped (overlap policy: skip)",
							tag.DAG(dagName),
						)
					case core.OverlapPolicyAll:
						// leave in buffer, retry next tick
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

		// Evaluate cron schedules for live runs
		for _, schedule := range entry.schedules {
			if schedule.Parsed == nil {
				continue
			}

			// Check if this schedule is due at this tick.
			// We check if the schedule's next time after (now - 1 second) is <= now,
			// matching the existing behavior in invokeJobs.
			next := schedule.Parsed.Next(now.Add(-time.Second))
			if next.After(now) {
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
	}

	tp.lastPlanResult = candidates
	return candidates
}

// shouldRun checks all guards for a live scheduled run.
func (tp *TickPlanner) shouldRun(ctx context.Context, dag *core.DAG, scheduledTime time.Time, schedule core.Schedule) bool {
	// Guard 1: isRunning (uses process-level check)
	if tp.cfg.IsRunning != nil {
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
	}

	if tp.cfg.GetLatestStatus == nil {
		return true
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

// computePrevExecTime calculates the previous schedule time from the given time
// by computing the schedule interval.
func computePrevExecTime(next time.Time, schedule core.Schedule) time.Time {
	nextNext := schedule.Parsed.Next(next.Add(time.Second))
	duration := nextNext.Sub(next)
	return next.Add(-duration)
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

	if tp.watermarkState == nil {
		return
	}

	tp.watermarkState.LastTick = now

	for _, run := range tp.lastPlanResult {
		if tp.watermarkState.DAGs == nil {
			tp.watermarkState.DAGs = make(map[string]DAGWatermark)
		}
		tp.watermarkState.DAGs[run.DAG.Name] = DAGWatermark{
			LastScheduledTime: run.ScheduledTime,
		}
	}

	tp.watermarkDirty.Store(true)
	tp.lastPlanResult = nil
}

// Flush writes the watermark state to disk if dirty.
// Safe for concurrent use.
func (tp *TickPlanner) Flush(ctx context.Context) {
	if tp.cfg.WatermarkStore == nil {
		return
	}
	if !tp.watermarkDirty.CompareAndSwap(true, false) {
		return
	}

	// Snapshot under read lock to avoid holding the lock during I/O.
	tp.mu.RLock()
	if tp.watermarkState == nil {
		tp.mu.RUnlock()
		return
	}
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

// StartFlusher runs the periodic watermark flusher. Blocks until ctx is done.
func (tp *TickPlanner) StartFlusher(ctx context.Context) {
	if tp.cfg.WatermarkStore == nil {
		return
	}

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

// drainEvents processes all queued DAG change events from the channel.
// Non-blocking: processes all available events, then returns.
func (tp *TickPlanner) drainEvents(ctx context.Context) {
	if tp.cfg.Events == nil {
		return
	}

	for {
		select {
		case event, ok := <-tp.cfg.Events:
			if !ok {
				return
			}
			tp.handleEvent(ctx, event)
		default:
			return
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
		tp.entries[event.DAGName] = &plannerEntry{
			dag:       event.DAG,
			schedules: event.DAG.Schedule,
		}
		// Set watermark to now (new DAGs have no catchup)
		tp.mu.Lock()
		if tp.watermarkState != nil {
			if tp.watermarkState.DAGs == nil {
				tp.watermarkState.DAGs = make(map[string]DAGWatermark)
			}
			tp.watermarkState.DAGs[event.DAGName] = DAGWatermark{
				LastScheduledTime: tp.cfg.Clock(),
			}
			tp.watermarkDirty.Store(true)
		}
		tp.mu.Unlock()
		logger.Info(ctx, "Planner: DAG added", tag.DAG(event.DAGName))

	case DAGChangeUpdated:
		if event.DAG == nil {
			return
		}
		tp.entries[event.DAGName] = &plannerEntry{
			dag:       event.DAG,
			schedules: event.DAG.Schedule,
		}
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
		if tp.watermarkState != nil {
			delete(tp.watermarkState.DAGs, event.DAGName)
			tp.watermarkDirty.Store(true)
		}
		tp.mu.Unlock()
		logger.Info(ctx, "Planner: DAG deleted", tag.DAG(event.DAGName))
	}
}

// recomputeBuffer creates a new catch-up buffer for a DAG using the existing watermark.
func (tp *TickPlanner) recomputeBuffer(ctx context.Context, dag *core.DAG) {
	tp.mu.RLock()
	state := tp.watermarkState
	tp.mu.RUnlock()

	if state == nil {
		return
	}

	now := tp.cfg.Clock()
	lastTick := state.LastTick
	var lastScheduledTime time.Time
	if wm, ok := state.DAGs[dag.Name]; ok {
		lastScheduledTime = wm.LastScheduledTime
	}

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
	tp.buffers[dag.Name] = q

	logger.Info(ctx, "Recomputed catch-up buffer",
		tag.DAG(dag.Name),
		slog.Int("missedCount", len(missed)),
	)
}

// DispatchRun dispatches a PlannedRun using the configured dispatch function.
func (tp *TickPlanner) DispatchRun(ctx context.Context, run PlannedRun) {
	logger.Info(ctx, "Dispatching planned run",
		tag.DAG(run.DAG.Name),
		tag.RunID(run.RunID),
		slog.String("triggerType", run.TriggerType.String()),
		slog.String("scheduledTime", run.ScheduledTime.Format(time.RFC3339)),
	)

	if err := tp.cfg.Dispatch(ctx, run.DAG, run.RunID, run.TriggerType); err != nil {
		logger.Error(ctx, "Failed to dispatch run",
			tag.DAG(run.DAG.Name),
			tag.RunID(run.RunID),
			tag.Error(err),
		)
	}
}

// HandleJobDirect dispatches a DAG execution directly (for stop/restart).
// This is a passthrough to the underlying dispatch function via the DAGExecutor.
func HandleJobDirect(
	ctx context.Context,
	dagExecutor *DAGExecutor,
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	triggerType core.TriggerType,
) error {
	return dagExecutor.HandleJob(ctx, dag, operation, runID, triggerType)
}
