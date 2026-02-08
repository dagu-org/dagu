package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

// CatchupEngine replays missed DAG runs that were scheduled during scheduler downtime.
type CatchupEngine struct {
	dagStateStore *DAGStateStore
	dagRunStore   exec.DAGRunStore
	dagExecutor   *DAGExecutor
	runtimeMgr    *runtime.Manager
	config        *config.Config
	clock         Clock
}

// CatchupResult summarizes what the catch-up engine did.
type CatchupResult struct {
	Dispatched int
	Skipped    int
	Duration   time.Duration
}

// catchupCandidate represents a single missed run to replay.
type catchupCandidate struct {
	dag           *core.DAG
	schedule      core.Schedule
	scheduledTime time.Time
}

// NewCatchupEngine creates a new CatchupEngine.
func NewCatchupEngine(
	dagStateStore *DAGStateStore,
	dagRunStore exec.DAGRunStore,
	dagExecutor *DAGExecutor,
	runtimeMgr *runtime.Manager,
	cfg *config.Config,
	clock Clock,
) *CatchupEngine {
	return &CatchupEngine{
		dagStateStore: dagStateStore,
		dagRunStore:   dagRunStore,
		dagExecutor:   dagExecutor,
		runtimeMgr:    runtimeMgr,
		config:        cfg,
		clock:         clock,
	}
}

// Run executes catch-up synchronously before the live scheduler loop.
func (c *CatchupEngine) Run(ctx context.Context, dags map[string]*core.DAG) (*CatchupResult, error) {
	start := c.clock()
	result := &CatchupResult{}

	perDAGStates, err := c.dagStateStore.LoadAll(dags)
	if err != nil {
		return nil, fmt.Errorf("failed to load per-DAG states: %w", err)
	}

	catchupTo := c.clock()

	// Check if any DAG has a non-zero watermark (i.e., has been seen before)
	hasAnyWatermark := false
	for _, state := range perDAGStates {
		if !state.LastTick.IsZero() {
			hasAnyWatermark = true
			break
		}
	}

	if !hasAnyWatermark {
		// First run — no catch-up needed, seed all DAGs with current time
		logger.Info(ctx, "No per-DAG watermarks found, skipping catch-up")
		if err := c.dagStateStore.SaveAll(dags, c.clock()); err != nil {
			return nil, fmt.Errorf("failed to save initial watermarks: %w", err)
		}
		result.Duration = c.clock().Sub(start)
		return result, nil
	}

	logger.Info(ctx, "Starting catch-up",
		slog.String("catchupTo", catchupTo.Format(time.RFC3339)),
	)

	// Generate candidates for all DAGs with catch-up enabled
	candidates := c.generateCandidates(ctx, dags, perDAGStates, catchupTo)

	if len(candidates) == 0 {
		logger.Info(ctx, "No catch-up candidates found")
		if err := c.dagStateStore.SaveAll(dags, catchupTo); err != nil {
			logger.Error(ctx, "Failed to save watermarks after catch-up", tag.Error(err))
		}
		result.Duration = c.clock().Sub(start)
		return result, nil
	}

	logger.Info(ctx, "Catch-up candidates generated",
		slog.Int("count", len(candidates)),
	)

	// Dispatch candidates
	for _, cand := range candidates {
		if ctx.Err() != nil {
			break
		}

		dispatched, err := c.dispatchCandidate(ctx, cand)
		if err != nil {
			logger.Error(ctx, "Catch-up dispatch failed, stopping catch-up",
				tag.DAG(cand.dag.Name),
				tag.Error(err),
			)
			// Save watermark at the last successful dispatch point
			break
		}

		if dispatched {
			result.Dispatched++
			// Advance this DAG's watermark after each successful dispatch
			if err := c.dagStateStore.Save(cand.dag, dagState{LastTick: cand.scheduledTime}); err != nil {
				logger.Error(ctx, "Failed to save DAG state", tag.Error(err))
			}
		} else {
			result.Skipped++
		}

		time.Sleep(c.config.Scheduler.CatchupRateLimit)
	}

	// Set watermarks to catchupTo after all dispatches
	if err := c.dagStateStore.SaveAll(dags, catchupTo); err != nil {
		logger.Error(ctx, "Failed to save final watermarks", tag.Error(err))
	}

	result.Duration = c.clock().Sub(start)

	logger.Info(ctx, "Catch-up completed",
		slog.Int("dispatched", result.Dispatched),
		slog.Int("skipped", result.Skipped),
		slog.String("duration", result.Duration.String()),
	)

	return result, nil
}

// generateCandidates generates all catch-up candidates across all DAGs,
// applies per-entry caps and policies, then applies global caps.
func (c *CatchupEngine) generateCandidates(
	ctx context.Context,
	dags map[string]*core.DAG,
	perDAGStates map[*core.DAG]dagState,
	catchupTo time.Time,
) []catchupCandidate {
	var merged []catchupCandidate

	for _, dag := range dags {
		lastTick := perDAGStates[dag].LastTick
		if lastTick.IsZero() {
			// First run for this DAG — no catch-up needed
			continue
		}

		var dagCands []catchupCandidate

		for _, sched := range dag.Schedule {
			if sched.Catchup == core.CatchupPolicyOff {
				continue
			}
			if sched.Parsed == nil {
				continue
			}

			entryCands := c.generateEntryCandidates(dag, sched, lastTick, catchupTo)
			entryCands = applyPolicy(sched.Catchup, entryCands)

			dagCands = append(dagCands, entryCands...)
		}

		if len(dagCands) == 0 {
			continue
		}

		// Sort by scheduled time within this DAG
		sort.Slice(dagCands, func(i, j int) bool {
			return dagCands[i].scheduledTime.Before(dagCands[j].scheduledTime)
		})

		// Apply per-DAG cap
		perDAGCap := c.config.Scheduler.MaxCatchupRunsPerDAG
		if perDAGCap > 0 && len(dagCands) > perDAGCap {
			logger.Info(ctx, "Capping catch-up runs for DAG",
				tag.DAG(dag.Name),
				slog.Int("candidates", len(dagCands)),
				slog.Int("cap", perDAGCap),
			)
			dagCands = dagCands[:perDAGCap]
		}

		merged = append(merged, dagCands...)
	}

	// Sort globally by scheduled time
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].scheduledTime.Before(merged[j].scheduledTime)
	})

	// Apply global cap
	if c.config.Scheduler.MaxGlobalCatchupRuns > 0 && len(merged) > c.config.Scheduler.MaxGlobalCatchupRuns {
		logger.Info(ctx, "Capping global catch-up runs",
			slog.Int("candidates", len(merged)),
			slog.Int("cap", c.config.Scheduler.MaxGlobalCatchupRuns),
		)
		merged = merged[:c.config.Scheduler.MaxGlobalCatchupRuns]
	}

	return merged
}

// generateEntryCandidates generates candidate times for a single schedule entry.
func (c *CatchupEngine) generateEntryCandidates(
	dag *core.DAG,
	sched core.Schedule,
	lastTick, catchupTo time.Time,
) []catchupCandidate {
	replayFrom := lastTick

	// Apply catchupWindow if configured
	if sched.CatchupWindow > 0 {
		windowStart := catchupTo.Add(-sched.CatchupWindow)
		if windowStart.After(replayFrom) {
			replayFrom = windowStart
		}
	}

	var candidates []catchupCandidate
	cursor := replayFrom

	for {
		next := sched.Parsed.Next(cursor)
		if next.After(catchupTo) || next.IsZero() {
			break
		}
		candidates = append(candidates, catchupCandidate{
			dag:           dag,
			schedule:      sched,
			scheduledTime: next,
		})
		cursor = next
	}

	return candidates
}

// applyPolicy filters candidates based on the catchup policy.
func applyPolicy(policy core.CatchupPolicy, candidates []catchupCandidate) []catchupCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	switch policy {
	case core.CatchupPolicyOff:
		return nil
	case core.CatchupPolicyLatest:
		return candidates[len(candidates)-1:] // latest
	case core.CatchupPolicyAll:
		return candidates
	default:
		return nil
	}
}

// dispatchCandidate dispatches a single catch-up run.
// Returns true if dispatched, false if skipped (duplicate).
func (c *CatchupEngine) dispatchCandidate(ctx context.Context, cand catchupCandidate) (bool, error) {
	if c.isDuplicate(ctx, cand) {
		logger.Info(ctx, "Skipping duplicate catch-up run",
			tag.DAG(cand.dag.Name),
			slog.String("scheduledTime", cand.scheduledTime.Format(time.RFC3339)),
		)
		return false, nil
	}

	runID, err := c.runtimeMgr.GenDAGRunID(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to generate run ID: %w", err)
	}

	logger.Info(ctx, "Dispatching catch-up run",
		tag.DAG(cand.dag.Name),
		tag.RunID(runID),
		slog.String("scheduledTime", cand.scheduledTime.Format(time.RFC3339)),
		slog.String("catchup", cand.schedule.Catchup.String()),
	)

	if err := c.dagExecutor.HandleJob(
		ctx,
		cand.dag,
		coordinatorv1.Operation_OPERATION_START,
		runID,
		core.TriggerTypeCatchUp,
		cand.scheduledTime,
	); err != nil {
		return false, fmt.Errorf("failed to dispatch catch-up run: %w", err)
	}

	return true, nil
}

// isDuplicate checks if a run already exists for the same DAG at the same scheduled time
// by comparing RFC3339 timestamps in recent attempts.
func (c *CatchupEngine) isDuplicate(ctx context.Context, cand catchupCandidate) bool {
	target := cand.scheduledTime.Format(time.RFC3339)
	attempts := c.dagRunStore.RecentAttempts(ctx, cand.dag.Name, 50)
	for _, attempt := range attempts {
		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			continue
		}
		if status.ScheduledTime == target {
			return true
		}
	}
	return false
}
