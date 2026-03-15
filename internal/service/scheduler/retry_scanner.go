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
	dagRunStore exec.DAGRunStore
	queueStore  exec.QueueStore
	isSuspended IsSuspendedFunc
	retryWindow time.Duration
	clock       Clock
}

func NewRetryScanner(
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

	failedRuns, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithStatuses([]core.Status{core.Failed}),
		exec.WithFrom(from),
		exec.WithoutLimit(),
	)
	if err != nil {
		return err
	}
	activeRuns, err := s.dagRunStore.ListStatuses(
		ctx,
		exec.WithStatuses([]core.Status{core.Running, core.Queued}),
		exec.WithFrom(from),
		exec.WithoutLimit(),
	)
	if err != nil {
		return err
	}

	activeRunIndex := latestActiveScheduleByName(activeRuns)
	for _, listed := range failedRuns {
		if listed == nil {
			continue
		}
		if err := s.processFailedRun(ctx, listed, activeRunIndex, now); err != nil {
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
	activeRunIndex map[string]time.Time,
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

	dagSnapshot, err := attempt.ReadDAG(ctx)
	if err != nil {
		return err
	}

	decision := s.evaluateRetryDecision(ctx, latestStatus, dagSnapshot, activeRunIndex, now)
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

	result, err := exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, dagSnapshot, latestStatus, exec.EnqueueRetryOptions{
		AutoRetry: true,
	})
	if err != nil {
		return err
	}

	switch result.Outcome {
	case exec.EnqueueRetryOutcomeQueued:
		queuedStatus := latestStatus
		if result.Status != nil {
			queuedStatus = result.Status
		}
		logger.Info(ctx, "Retry scanner enqueued DAG-level retry",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
			slog.Int("auto_retry_count", queuedStatus.AutoRetryCount),
			slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
			slog.Duration("computed_delay", decision.computedDelay),
		)
	case exec.EnqueueRetryOutcomeAlreadyQueued:
		logger.Debug(ctx, "Retry scanner found DAG run already queued for retry",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
		)
	case exec.EnqueueRetryOutcomeStaleLatest:
		logger.Debug(ctx, "Retry scanner skipped DAG run",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
			slog.String("skip_reason", "stale_latest"),
		)
	}
	return nil
}

func (s *RetryScanner) evaluateRetryDecision(
	ctx context.Context,
	status *exec.DAGRunStatus,
	dagSnapshot *core.DAG,
	activeRunIndex map[string]time.Time,
	now time.Time,
) retryDecision {
	if dagSnapshot == nil || dagSnapshot.RetryPolicy == nil {
		return retryDecision{reason: "retry_policy_missing"}
	}
	if s.isSuspended(ctx, dagSuspendFlagName(dagSnapshot)) {
		return retryDecision{reason: "suspended"}
	}
	if newerScheduledRunExists(status, activeRunIndex) {
		return retryDecision{reason: "newer_run_exists"}
	}
	if status.AutoRetryCount >= dagSnapshot.RetryPolicy.Limit {
		return retryDecision{reason: "retry_exhausted"}
	}

	finishedAt, ok := parseRFC3339(status.FinishedAt)
	if !ok {
		return retryDecision{reason: "missing_finished_at"}
	}

	delay := dagRetryDelay(dagSnapshot.RetryPolicy, status.AutoRetryCount)
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

func latestActiveScheduleByName(statuses []*exec.DAGRunStatus) map[string]time.Time {
	grouped := make(map[string]time.Time, len(statuses))
	for _, status := range statuses {
		if status == nil {
			continue
		}
		scheduleTime, ok := parseRFC3339(status.ScheduleTime)
		if !ok {
			continue
		}
		if current, exists := grouped[status.Name]; !exists || scheduleTime.After(current) {
			grouped[status.Name] = scheduleTime
		}
	}
	return grouped
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

func newerScheduledRunExists(failed *exec.DAGRunStatus, activeRunIndex map[string]time.Time) bool {
	failedSchedule, ok := parseRFC3339(failed.ScheduleTime)
	if !ok {
		return false
	}
	activeSchedule, ok := activeRunIndex[failed.Name]
	return ok && activeSchedule.After(failedSchedule)
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
