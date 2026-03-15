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
	_ EntryReader,
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

	activeRunsByName := groupStatusesByName(activeRuns)
	for _, listed := range failedRuns {
		if listed == nil {
			continue
		}
		if err := s.processFailedRun(ctx, listed, activeRunsByName[listed.Name], now); err != nil {
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
	activeRuns []*exec.DAGRunStatus,
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

	decision := s.evaluateRetryDecision(ctx, latestStatus, dagSnapshot, activeRuns, now)
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

	if err := exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, dagSnapshot, latestStatus, exec.EnqueueRetryOptions{
		AutoRetry: true,
	}); err != nil {
		return err
	}

	queuedStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		queuedStatus = latestStatus
	}

	logger.Info(ctx, "Retry scanner enqueued DAG-level retry",
		tag.DAG(latestStatus.Name),
		tag.RunID(latestStatus.DAGRunID),
		slog.Int("auto_retry_count", queuedStatus.AutoRetryCount),
		slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
		slog.Duration("computed_delay", decision.computedDelay),
	)
	return nil
}

func (s *RetryScanner) evaluateRetryDecision(
	ctx context.Context,
	status *exec.DAGRunStatus,
	dagSnapshot *core.DAG,
	activeRuns []*exec.DAGRunStatus,
	now time.Time,
) retryDecision {
	if dagSnapshot == nil || dagSnapshot.RetryPolicy == nil {
		return retryDecision{reason: "retry_policy_missing"}
	}
	if s.isSuspended(ctx, dagSuspendFlagName(dagSnapshot)) {
		return retryDecision{reason: "suspended"}
	}
	if newerScheduledRunExists(status, activeRuns) {
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

func groupStatusesByName(statuses []*exec.DAGRunStatus) map[string][]*exec.DAGRunStatus {
	grouped := make(map[string][]*exec.DAGRunStatus, len(statuses))
	for _, status := range statuses {
		if status == nil {
			continue
		}
		grouped[status.Name] = append(grouped[status.Name], status)
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

func newerScheduledRunExists(failed *exec.DAGRunStatus, activeRuns []*exec.DAGRunStatus) bool {
	failedSchedule, ok := parseRFC3339(failed.ScheduleTime)
	if !ok {
		return false
	}

	for _, candidate := range activeRuns {
		if candidate == nil || candidate.DAGRunID == failed.DAGRunID {
			continue
		}
		if candidate.Name != failed.Name {
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
