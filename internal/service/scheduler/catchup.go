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
	watermarkStore *WatermarkStore
	dagRunStore    exec.DAGRunStore
	dagExecutor    *DAGExecutor
	runtimeMgr     *runtime.Manager
	config         *config.Config
	location       *time.Location
	clock          Clock
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
	watermarkStore *WatermarkStore,
	dagRunStore exec.DAGRunStore,
	dagExecutor *DAGExecutor,
	runtimeMgr *runtime.Manager,
	cfg *config.Config,
	location *time.Location,
	clock Clock,
) *CatchupEngine {
	return &CatchupEngine{
		watermarkStore: watermarkStore,
		dagRunStore:    dagRunStore,
		dagExecutor:    dagExecutor,
		runtimeMgr:     runtimeMgr,
		config:         cfg,
		location:       location,
		clock:          clock,
	}
}

// Run executes catch-up synchronously before the live scheduler loop.
func (c *CatchupEngine) Run(ctx context.Context, dags map[string]*core.DAG) (*CatchupResult, error) {
	start := c.clock()
	result := &CatchupResult{}

	lastTick, err := c.watermarkStore.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load watermark: %w", err)
	}

	// If no watermark exists (first run or corrupt), set watermark to now and skip catch-up.
	if lastTick.IsZero() {
		logger.Info(ctx, "No scheduler watermark found, skipping catch-up")
		if err := c.watermarkStore.Save(c.clock()); err != nil {
			return nil, fmt.Errorf("failed to save initial watermark: %w", err)
		}
		result.Duration = c.clock().Sub(start)
		return result, nil
	}

	catchupTo := c.clock()

	logger.Info(ctx, "Starting catch-up",
		slog.String("lastTick", lastTick.Format(time.RFC3339)),
		slog.String("catchupTo", catchupTo.Format(time.RFC3339)),
		slog.String("gap", catchupTo.Sub(lastTick).String()),
	)

	// Generate candidates for all DAGs with catch-up enabled
	candidates := c.generateCandidates(ctx, dags, lastTick, catchupTo)

	if len(candidates) == 0 {
		logger.Info(ctx, "No catch-up candidates found")
		if err := c.watermarkStore.Save(catchupTo); err != nil {
			logger.Error(ctx, "Failed to save watermark after catch-up", tag.Error(err))
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
			// Advance watermark after each successful dispatch
			if err := c.watermarkStore.Save(cand.scheduledTime); err != nil {
				logger.Error(ctx, "Failed to save watermark", tag.Error(err))
			}
		} else {
			result.Skipped++
		}

		// Rate limit
		time.Sleep(c.config.Scheduler.CatchupRateLimit)
	}

	// Set watermark to catchupTo after all dispatches
	if err := c.watermarkStore.Save(catchupTo); err != nil {
		logger.Error(ctx, "Failed to save final watermark", tag.Error(err))
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
	lastTick, catchupTo time.Time,
) []catchupCandidate {
	type dagCandidates struct {
		dagName    string
		candidates []catchupCandidate
	}

	var allDAGCandidates []dagCandidates

	for _, dag := range dags {
		var dagCands []catchupCandidate

		for _, sched := range dag.Schedule {
			if sched.Misfire == core.MisfirePolicyIgnore {
				continue
			}
			if sched.Parsed == nil {
				continue
			}

			entryCands := c.generateEntryCandidates(dag, sched, lastTick, catchupTo)
			entryCands = c.applyPolicy(sched.Misfire, entryCands)

			// Apply per-entry maxCatchupRuns cap
			if sched.MaxCatchupRuns > 0 && len(entryCands) > sched.MaxCatchupRuns {
				entryCands = entryCands[:sched.MaxCatchupRuns]
			}

			dagCands = append(dagCands, entryCands...)
		}

		if len(dagCands) > 0 {
			// Sort by scheduled time within this DAG
			sort.Slice(dagCands, func(i, j int) bool {
				return dagCands[i].scheduledTime.Before(dagCands[j].scheduledTime)
			})

			// Apply per-DAG cap
			if c.config.Scheduler.MaxCatchupRunsPerDAG > 0 && len(dagCands) > c.config.Scheduler.MaxCatchupRunsPerDAG {
				logger.Info(ctx, "Capping catch-up runs for DAG",
					tag.DAG(dag.Name),
					slog.Int("candidates", len(dagCands)),
					slog.Int("cap", c.config.Scheduler.MaxCatchupRunsPerDAG),
				)
				dagCands = dagCands[:c.config.Scheduler.MaxCatchupRunsPerDAG]
			}

			allDAGCandidates = append(allDAGCandidates, dagCandidates{
				dagName:    dag.Name,
				candidates: dagCands,
			})
		}
	}

	// Merge all candidates and sort globally by scheduled time
	var merged []catchupCandidate
	for _, dc := range allDAGCandidates {
		merged = append(merged, dc.candidates...)
	}
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

// applyPolicy filters candidates based on the misfire policy.
func (c *CatchupEngine) applyPolicy(policy core.MisfirePolicy, candidates []catchupCandidate) []catchupCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	switch policy {
	case core.MisfirePolicyIgnore:
		return nil
	case MisfirePolicyRunOnce:
		return candidates[:1] // earliest
	case MisfirePolicyRunLatest:
		return candidates[len(candidates)-1:] // latest
	case MisfirePolicyRunAll:
		return candidates // all
	default:
		return nil // unknown policy, treat as ignore
	}
}

// dispatchCandidate dispatches a single catch-up run.
// Returns true if dispatched, false if skipped (duplicate).
func (c *CatchupEngine) dispatchCandidate(ctx context.Context, cand catchupCandidate) (bool, error) {
	// Check for duplicate: is there already a run for this DAG at this scheduled time?
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
		slog.String("misfire", cand.schedule.Misfire.String()),
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

// isDuplicate checks if a run already exists for the same DAG at the same scheduled time.
// It looks at recent runs within the scheduled time's minute window.
func (c *CatchupEngine) isDuplicate(ctx context.Context, cand catchupCandidate) bool {
	attempts := c.dagRunStore.RecentAttempts(ctx, cand.dag.Name, 50)
	for _, attempt := range attempts {
		status, err := attempt.ReadStatus(ctx)
		if err != nil {
			continue
		}
		if status.ScheduledTime != "" && status.ScheduledTime == cand.scheduledTime.Format(time.RFC3339) {
			return true
		}
	}
	return false
}

// Policy constants aliased for use in applyPolicy.
const (
	MisfirePolicyRunOnce  = core.MisfirePolicyRunOnce
	MisfirePolicyRunLatest = core.MisfirePolicyRunLatest
	MisfirePolicyRunAll   = core.MisfirePolicyRunAll
)
