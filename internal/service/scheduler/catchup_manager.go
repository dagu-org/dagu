package scheduler

import (
	"context"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
)

// DispatchFunc dispatches a catch-up or scheduled run for the given DAG.
type DispatchFunc func(ctx context.Context, dag *core.DAG, runID string, triggerType core.TriggerType) error

// RunIDFunc generates a unique run ID.
type RunIDFunc func(ctx context.Context) (string, error)

// IsRunningFunc checks if a DAG has any active run.
type IsRunningFunc func(ctx context.Context, dag *core.DAG) (bool, error)

// CatchupManagerConfig holds the configuration for creating a CatchupManager.
type CatchupManagerConfig struct {
	WatermarkStore core.WatermarkStore
	Dispatch       DispatchFunc
	GenRunID       RunIDFunc
	IsRunning      IsRunningFunc
	Clock          Clock
}

// CatchupManager handles watermark persistence and catch-up buffer processing.
//
// Thread safety: AdvanceWatermark, ProcessBuffers, and RouteToBuffer are called
// from the cronLoop goroutine. StartFlusher and Flush may be called from other
// goroutines. The internal RWMutex protects watermarkState for concurrent access.
type CatchupManager struct {
	watermarkStore core.WatermarkStore
	dispatch       DispatchFunc
	genRunID       RunIDFunc
	isRunning      IsRunningFunc
	clock          Clock

	// mu protects watermarkState for concurrent access between
	// the cronLoop (writer) and the flusher goroutine (reader).
	mu              sync.RWMutex
	watermarkState  *core.SchedulerState
	watermarkDirty  atomic.Bool
	scheduleBuffers map[string]*ScheduleBuffer
}

// NewCatchupManager creates a new CatchupManager.
func NewCatchupManager(cfg CatchupManagerConfig) *CatchupManager {
	return &CatchupManager{
		watermarkStore: cfg.WatermarkStore,
		dispatch:       cfg.Dispatch,
		genRunID:       cfg.GenRunID,
		isRunning:      cfg.IsRunning,
		clock:          cfg.Clock,
	}
}

// Init loads the watermark from persistent storage and populates catch-up
// buffers for DAGs with CatchupWindow > 0. Must be called before ProcessBuffers.
func (cm *CatchupManager) Init(ctx context.Context, dags []*core.DAG) error {
	if cm.watermarkStore == nil {
		return nil
	}

	state, err := cm.watermarkStore.Load(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to load watermark state", tag.Error(err))
		// Non-fatal: continue without catch-up
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

	cm.mu.Lock()
	cm.watermarkState = state
	cm.mu.Unlock()

	logger.Info(ctx, "Loaded scheduler watermark",
		slog.Time("lastTick", state.LastTick),
		slog.Int("dagCount", len(state.DAGs)),
	)

	cm.initBuffers(ctx, dags)
	return nil
}

// initBuffers creates per-DAG queues for DAGs with CatchupWindow > 0
// and enqueues catch-up items.
func (cm *CatchupManager) initBuffers(ctx context.Context, dags []*core.DAG) {
	cm.mu.RLock()
	state := cm.watermarkState
	cm.mu.RUnlock()

	if state == nil {
		return
	}

	now := cm.clock()
	buffers := make(map[string]*ScheduleBuffer, len(dags))
	var totalMissed int

	for _, dag := range dags {
		if dag.CatchupWindow <= 0 {
			continue
		}

		dagName := dag.Name

		var lastTick time.Time
		var lastScheduledTime time.Time

		lastTick = state.LastTick
		if wm, ok := state.DAGs[dagName]; ok {
			lastScheduledTime = wm.LastScheduledTime
		}

		replayFrom := ComputeReplayFrom(dag.CatchupWindow, lastTick, lastScheduledTime, now)
		missed := ComputeMissedIntervals(dag.Schedule, replayFrom, now)

		if len(missed) == 0 {
			continue
		}

		totalMissed += len(missed)

		logger.Info(ctx, "Catch-up planned",
			tag.DAG(dagName),
			slog.Int("missedCount", len(missed)),
			slog.Time("replayFrom", replayFrom),
			slog.Time("replayTo", now),
		)

		q := NewScheduleBuffer(dagName, dag.OverlapPolicy)
		buffers[dagName] = q

		for _, t := range missed {
			if !q.Send(QueueItem{
				DAG:           dag,
				ScheduledTime: t,
				TriggerType:   core.TriggerTypeCatchUp,
				ScheduleType:  ScheduleTypeStart,
			}) {
				logger.Error(ctx, "Catch-up buffer full, dropping remaining items",
					tag.DAG(dagName),
					slog.Int("buffered", q.Len()),
					slog.Int("dropped", len(missed)-q.Len()),
				)
				break
			}
		}
	}

	cm.mu.Lock()
	cm.scheduleBuffers = buffers
	cm.mu.Unlock()

	if totalMissed > 0 {
		logger.Info(ctx, "Catch-up initialization complete",
			slog.Int("dagCount", len(buffers)),
			slog.Int("totalMissedRuns", totalMissed),
		)
	}
}

// AdvanceWatermark updates the last tick time in the in-memory watermark state.
// Called from cronLoop after each tick.
func (cm *CatchupManager) AdvanceWatermark(tickTime time.Time) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.watermarkState == nil {
		return
	}
	cm.watermarkState.LastTick = tickTime
	cm.watermarkDirty.Store(true)
}

// ProcessBuffers drains one item per buffer each tick, respecting overlap policy.
// Called from cronLoop.
func (cm *CatchupManager) ProcessBuffers(ctx context.Context) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for dagName, buf := range cm.scheduleBuffers {
		item, ok := buf.Peek()
		if !ok {
			// Buffer fully drained — remove it from the map.
			delete(cm.scheduleBuffers, dagName)
			continue
		}

		running, err := cm.isRunning(ctx, item.DAG)
		if err != nil {
			// Treat as "not running" so the buffer keeps draining.
			// If the DAG was deleted, dispatch will fail and the item is gone.
			// If transient, the execution layer handles overlap.
			logger.Error(ctx, "Failed to check if DAG is running, assuming not running",
				tag.DAG(dagName),
				tag.Error(err),
			)
			running = false
		}

		if !running {
			buf.Pop()
			cm.dispatchItem(ctx, item)
			continue
		}

		// DAG is running - apply overlap policy
		switch buf.overlapPolicy {
		case core.OverlapPolicySkip:
			buf.Pop() // discard
			logger.Info(ctx, "Catch-up run skipped (overlap policy: skip)",
				tag.DAG(dagName),
			)
		case core.OverlapPolicyAll:
			// leave in queue, retry next tick
		}
	}
}

// RouteToBuffer attempts to route a scheduled start job to its catch-up buffer.
// Returns true if the item was buffered, false if no buffer exists for this DAG
// or the buffer is full (caller should dispatch directly as fallback).
// Called from invokeJobs in the cronLoop.
func (cm *CatchupManager) RouteToBuffer(dagName string, item QueueItem) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	if cm.scheduleBuffers == nil {
		return false
	}
	q, ok := cm.scheduleBuffers[dagName]
	if !ok {
		return false
	}
	return q.Send(item)
}

// StartFlusher runs the periodic watermark flusher. Blocks until ctx is done.
func (cm *CatchupManager) StartFlusher(ctx context.Context) {
	if cm.watermarkStore == nil {
		return
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cm.Flush(ctx)
		}
	}
}

// Flush writes the watermark state to disk if dirty.
// Safe for concurrent use.
func (cm *CatchupManager) Flush(ctx context.Context) {
	if cm.watermarkStore == nil {
		return
	}
	if !cm.watermarkDirty.CompareAndSwap(true, false) {
		return
	}

	// Snapshot under read lock to avoid concurrent map access during Save.
	cm.mu.RLock()
	if cm.watermarkState == nil {
		cm.mu.RUnlock()
		return
	}
	snapshot := &core.SchedulerState{
		Version:  cm.watermarkState.Version,
		LastTick: cm.watermarkState.LastTick,
		DAGs:     make(map[string]core.DAGWatermark, len(cm.watermarkState.DAGs)),
	}
	maps.Copy(snapshot.DAGs, cm.watermarkState.DAGs)
	cm.mu.RUnlock()

	if err := cm.watermarkStore.Save(ctx, snapshot); err != nil {
		logger.Error(ctx, "Failed to flush watermark state", tag.Error(err))
		// Re-mark dirty so we try again next cycle
		cm.watermarkDirty.Store(true)
	}
}

// dispatchItem dispatches a single buffer item and updates the per-DAG watermark.
// Must be called with cm.mu held.
func (cm *CatchupManager) dispatchItem(ctx context.Context, item QueueItem) {
	runID, err := cm.genRunID(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to generate run ID for catch-up dispatch",
			tag.DAG(item.DAG.Name),
			tag.Error(err),
		)
		return
	}

	logger.Info(ctx, "Dispatching queued run",
		tag.DAG(item.DAG.Name),
		tag.RunID(runID),
		slog.String("triggerType", item.TriggerType.String()),
		slog.String("scheduledTime", item.ScheduledTime.Format(time.RFC3339)),
	)

	err = cm.dispatch(ctx, item.DAG, runID, item.TriggerType)
	if err != nil {
		logger.Error(ctx, "Failed to dispatch catch-up run",
			tag.DAG(item.DAG.Name),
			tag.Error(err),
		)
		return
	}

	cm.updateWatermarkDAG(item.DAG.Name, item.ScheduledTime)
}

// updateWatermarkDAG updates the per-DAG watermark after a dispatch.
// Must be called with cm.mu held.
func (cm *CatchupManager) updateWatermarkDAG(dagName string, scheduledTime time.Time) {
	if cm.watermarkState == nil {
		return
	}
	if cm.watermarkState.DAGs == nil {
		cm.watermarkState.DAGs = make(map[string]core.DAGWatermark)
	}
	cm.watermarkState.DAGs[dagName] = core.DAGWatermark{
		LastScheduledTime: scheduledTime,
	}
	cm.watermarkDirty.Store(true)
}
