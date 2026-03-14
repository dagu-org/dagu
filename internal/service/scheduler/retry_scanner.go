// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const retryScanInterval = 15 * time.Second

type retryDecision struct {
	enqueue       bool
	reason        string
	computedDelay time.Duration
	nextRetryAt   time.Time
}

// RetryScanner periodically discovers failed latest attempts and enqueues
// DAG-level retries once their backoff has elapsed.
type RetryScanner struct {
	entryReader EntryReader
	dagRunStore exec.DAGRunStore
	queueStore  exec.QueueStore
	isSuspended IsSuspendedFunc
	retryWindow time.Duration
	clock       Clock
}

func NewRetryScanner(
	entryReader EntryReader,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	isSuspended IsSuspendedFunc,
	retryWindow time.Duration,
	clock Clock,
) (*RetryScanner, error) {
	if clock == nil {
		clock = time.Now
	}
	if isSuspended == nil {
		isSuspended = func(context.Context, string) bool { return false }
	}
	return &RetryScanner{
		entryReader: entryReader,
		dagRunStore: dagRunStore,
		queueStore:  queueStore,
		isSuspended: isSuspended,
		retryWindow: retryWindow,
		clock:       clock,
	}, nil
}

func (s *RetryScanner) Start(ctx context.Context) {
	if s == nil || s.retryWindow <= 0 {
		return
	}

	if err := s.scan(ctx); err != nil {
		logger.Error(ctx, "Retry scanner scan failed", tag.Error(err))
	}

	ticker := time.NewTicker(retryScanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := s.scan(ctx); err != nil {
				logger.Error(ctx, "Retry scanner scan failed", tag.Error(err))
			}
		}
	}
}

func (s *RetryScanner) scan(ctx context.Context) error {
	now := s.clock().UTC()
	from := exec.NewUTC(now.Add(-s.retryWindow))

	for _, dag := range s.retryEnabledDAGs() {
		if dag == nil {
			continue
		}
		if err := s.scanDAG(ctx, dag, from, now); err != nil {
			logger.Error(ctx, "Retry scanner failed to process DAG",
				tag.DAG(dag.Name),
				tag.Error(err),
			)
		}
	}

	return nil
}

func (s *RetryScanner) retryEnabledDAGs() []*core.DAG {
	dags := s.entryReader.DAGs()
	result := make([]*core.DAG, 0, len(dags))
	seen := make(map[string]struct{}, len(dags))
	for _, dag := range dags {
		if dag == nil || dag.RetryPolicy == nil {
			continue
		}
		if _, ok := seen[dag.Name]; ok {
			continue
		}
		seen[dag.Name] = struct{}{}
		result = append(result, dag)
	}
	return result
}

func (s *RetryScanner) scanDAG(ctx context.Context, dag *core.DAG, from exec.TimeInUTC, now time.Time) error {
	failedRuns, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithExactName(dag.Name),
		exec.WithStatuses([]core.Status{core.Failed}),
		exec.WithFrom(from),
		exec.WithoutLimit(),
	)
	if err != nil {
		return err
	}
	if len(failedRuns) == 0 {
		return nil
	}

	activeRuns, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithExactName(dag.Name),
		exec.WithStatuses([]core.Status{core.Running, core.Queued}),
		exec.WithFrom(from),
		exec.WithoutLimit(),
	)
	if err != nil {
		return err
	}

	suspended := s.isSuspended(ctx, dagSuspendFlagName(dag))
	for _, listed := range failedRuns {
		if listed == nil {
			continue
		}
		if err := s.processFailedRun(ctx, listed, dag, activeRuns, suspended, now); err != nil {
			logger.Error(ctx, "Retry scanner failed to process DAG run",
				tag.DAG(listed.Name),
				tag.RunID(listed.DAGRunID),
				tag.Error(err),
			)
		}
	}
	return nil
}

func (s *RetryScanner) processFailedRun(
	ctx context.Context,
	listed *exec.DAGRunStatus,
	currentDAG *core.DAG,
	activeRuns []*exec.DAGRunStatus,
	suspended bool,
	now time.Time,
) error {
	ref := listed.DAGRun()
	attempt, err := s.dagRunStore.FindAttempt(ctx, ref)
	if err != nil {
		return err
	}

	latestStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		return err
	}
	if latestStatus.AttemptID != listed.AttemptID || latestStatus.Status != core.Failed {
		return nil
	}
	if !latestStatus.Parent.Zero() {
		return nil
	}

	decision := s.evaluateRetryDecision(latestStatus, currentDAG, activeRuns, suspended, now)
	if !decision.enqueue {
		if decision.reason != "" {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(latestStatus.Name),
				tag.RunID(latestStatus.DAGRunID),
				slog.String("skip_reason", decision.reason),
			)
		}
		return nil
	}

	dagSnapshot, err := attempt.ReadDAG(ctx)
	if err != nil {
		return err
	}
	if err := exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, dagSnapshot, latestStatus); err != nil {
		return err
	}

	logger.Info(ctx, "Retry scanner enqueued DAG-level retry",
		tag.DAG(latestStatus.Name),
		tag.RunID(latestStatus.DAGRunID),
		slog.Int("retry_count", latestStatus.RetryCount),
		slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
		slog.Duration("computed_delay", decision.computedDelay),
	)
	return nil
}

func (s *RetryScanner) evaluateRetryDecision(
	status *exec.DAGRunStatus,
	currentDAG *core.DAG,
	activeRuns []*exec.DAGRunStatus,
	suspended bool,
	now time.Time,
) retryDecision {
	if currentDAG == nil || currentDAG.RetryPolicy == nil {
		return retryDecision{reason: "retry_policy_missing"}
	}
	if suspended {
		return retryDecision{reason: "suspended"}
	}
	if newerScheduledRunExists(status, activeRuns) {
		return retryDecision{reason: "newer_run_exists"}
	}
	if status.RetryCount >= currentDAG.RetryPolicy.Limit {
		return retryDecision{reason: "retry_exhausted"}
	}

	finishedAt, ok := parseRFC3339(status.FinishedAt)
	if !ok {
		return retryDecision{reason: "missing_finished_at"}
	}

	delay := dagRetryDelay(currentDAG.RetryPolicy, status.RetryCount)
	nextRetryAt := finishedAt.Add(delay)
	if now.Before(nextRetryAt) {
		return retryDecision{
			reason:        "backoff_not_elapsed",
			computedDelay: delay,
			nextRetryAt:   nextRetryAt,
		}
	}

	return retryDecision{
		enqueue:       true,
		computedDelay: delay,
		nextRetryAt:   nextRetryAt,
	}
}

func dagRetryDelay(policy *core.DAGRetryPolicy, retryCount int) time.Duration {
	if policy == nil {
		return 0
	}
	base := float64(policy.Interval)
	delay := base * math.Pow(policy.Backoff, float64(retryCount))
	if delay > float64(policy.MaxInterval) {
		delay = float64(policy.MaxInterval)
	}
	return time.Duration(delay)
}

func newerScheduledRunExists(failed *exec.DAGRunStatus, activeRuns []*exec.DAGRunStatus) bool {
	failedSchedule, ok := parseRFC3339(failed.ScheduleTime)
	if !ok {
		return false
	}

	for _, candidate := range activeRuns {
		if candidate == nil || candidate.DAGRunID == failed.DAGRunID {
			continue
		}
		activeSchedule, ok := parseRFC3339(candidate.ScheduleTime)
		if !ok {
			continue
		}
		if activeSchedule.After(failedSchedule) {
			return true
		}
	}
	return false
}

func parseRFC3339(val string) (time.Time, bool) {
	if val == "" {
		return time.Time{}, false
	}
	parsed, err := time.Parse(time.RFC3339, val)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}
