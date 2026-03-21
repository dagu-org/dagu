// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

const retryScanInterval = 30 * time.Second

type retryDecision struct {
	enqueue       bool
	reason        string
	computedDelay time.Duration
	nextRetryAt   time.Time
}

type dagRetryMetadata struct {
	limit       int
	interval    time.Duration
	backoff     float64
	maxInterval time.Duration
}

// RetryScanner periodically discovers failed latest attempts and enqueues
// DAG-level retries once their backoff has elapsed.
type RetryScanner struct {
	dagRunStore exec.DAGRunStore
	queueStore  exec.QueueStore
	isSuspended IsSuspendedFunc
	retryWindow time.Duration
	clock       Clock
	listTargets func() []string
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
	targets := s.retryTargetNames()
	for _, name := range targets {
		if err := s.processLatestAttempt(ctx, name, now); err != nil {
			logger.Error(ctx, "Retry scanner failed to process DAG run",
				tag.DAG(name),
				tag.Error(err),
			)
		}
	}
	return nil
}

func (s *RetryScanner) processLatestAttempt(ctx context.Context, dagName string, now time.Time) error {
	attempt, err := s.dagRunStore.LatestAttempt(ctx, dagName)
	if err != nil {
		if errors.Is(err, exec.ErrDAGRunIDNotFound) || errors.Is(err, exec.ErrNoStatusData) {
			return nil
		}
		return err
	}

	latestStatus, err := attempt.ReadStatus(ctx)
	if err != nil {
		if errors.Is(err, exec.ErrNoStatusData) {
			return nil
		}
		return err
	}
	if latestStatus.Status != core.Failed || !latestStatus.Parent.Zero() {
		return nil
	}

	referenceTime, ok := retryReferenceTime(latestStatus)
	if !ok {
		logger.Debug(ctx, "Retry scanner skipped DAG run",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
			slog.String("skip_reason", "missing_retry_reference_time"),
		)
		return nil
	}
	if s.retryWindow > 0 && referenceTime.Before(now.Add(-s.retryWindow)) {
		logger.Debug(ctx, "Retry scanner skipped DAG run",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
			slog.String("skip_reason", "outside_retry_window"),
		)
		return nil
	}

	metadata, ok := retryMetadataFromStatus(latestStatus)
	var dagSnapshot *core.DAG
	if !ok {
		dagSnapshot, err = attempt.ReadDAG(ctx)
		if err != nil {
			return err
		}
		metadata, ok = retryMetadataFromDAG(dagSnapshot)
		if !ok {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(latestStatus.Name),
				tag.RunID(latestStatus.DAGRunID),
				slog.String("skip_reason", "retry_policy_missing"),
			)
			return nil
		}
	}

	decision := s.evaluateRetryDecision(ctx, latestStatus, metadata, now)
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

	err = exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, dagSnapshot, latestStatus, exec.EnqueueRetryOptions{
		AutoRetry: true,
	})
	if err != nil {
		if errors.Is(err, exec.ErrRetryStaleLatest) {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(latestStatus.Name),
				tag.RunID(latestStatus.DAGRunID),
				slog.String("skip_reason", "stale_latest"),
			)
			return nil
		}
		return err
	}

	logger.Info(ctx, "Retry scanner ensured DAG-level retry is queued",
		tag.DAG(latestStatus.Name),
		tag.RunID(latestStatus.DAGRunID),
		slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
		slog.Duration("computed_delay", decision.computedDelay),
	)
	return nil
}

func (s *RetryScanner) listFailedRuns(ctx context.Context, from exec.TimeInUTC) ([]*exec.DAGRunStatus, error) {
	baseOpts := []exec.ListDAGRunStatusesOption{
		exec.WithStatuses([]core.Status{core.Failed}),
		exec.WithFrom(from),
		exec.WithoutLimit(),
	}

	if s.listTargets == nil {
		return s.dagRunStore.ListStatuses(ctx, baseOpts...)
	}

	targets := s.retryTargetNames()
	if len(targets) == 0 {
		return nil, nil
	}

	var failedRuns []*exec.DAGRunStatus
	for _, name := range targets {
		opts := append([]exec.ListDAGRunStatusesOption{}, baseOpts...)
		opts = append(opts, exec.WithExactName(name))
		items, err := s.dagRunStore.ListStatuses(ctx, opts...)
		if err != nil {
			return nil, err
		}
		failedRuns = append(failedRuns, items...)
	}

	return failedRuns, nil
}

func (s *RetryScanner) retryTargetNames() []string {
	if s.listTargets == nil {
		return nil
	}
	raw := s.listTargets()
	if len(raw) == 0 {
		return nil
	}

	targets := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, name := range raw {
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		targets = append(targets, name)
	}
	sort.Strings(targets)
	return targets
}

func (s *RetryScanner) processFailedRun(
	ctx context.Context,
	listed *exec.DAGRunStatus,
	now time.Time,
) error {
	if listed == nil {
		return nil
	}
	if metadata, ok := retryMetadataFromStatus(listed); ok {
		return s.processFailedRunFromSummary(ctx, listed, metadata, now)
	}
	return s.processFailedRunLegacy(ctx, listed, now)
}

func (s *RetryScanner) processFailedRunFromSummary(
	ctx context.Context,
	listed *exec.DAGRunStatus,
	metadata dagRetryMetadata,
	now time.Time,
) error {
	if !listed.Parent.Zero() {
		return nil
	}

	decision := s.evaluateRetryDecision(ctx, listed, metadata, now)
	if !decision.enqueue {
		if decision.reason != "" {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(listed.Name),
				tag.RunID(listed.DAGRunID),
				slog.String("skip_reason", decision.reason),
			)
		}
		return nil
	}

	err := exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, nil, listed, exec.EnqueueRetryOptions{
		AutoRetry: true,
	})
	if err != nil {
		if errors.Is(err, exec.ErrRetryStaleLatest) {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(listed.Name),
				tag.RunID(listed.DAGRunID),
				slog.String("skip_reason", "stale_latest"),
			)
			return nil
		}
		return err
	}

	logger.Info(ctx, "Retry scanner ensured DAG-level retry is queued",
		tag.DAG(listed.Name),
		tag.RunID(listed.DAGRunID),
		slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
		slog.Duration("computed_delay", decision.computedDelay),
	)
	return nil
}

func (s *RetryScanner) processFailedRunLegacy(
	ctx context.Context,
	listed *exec.DAGRunStatus,
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

	metadata, ok := retryMetadataFromDAG(dagSnapshot)
	if !ok {
		decision := retryDecision{reason: "retry_policy_missing"}
		logger.Debug(ctx, "Retry scanner skipped DAG run",
			tag.DAG(latestStatus.Name),
			tag.RunID(latestStatus.DAGRunID),
			slog.String("skip_reason", decision.reason),
		)
		return nil
	}

	decision := s.evaluateRetryDecision(ctx, latestStatus, metadata, now)
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

	err = exec.EnqueueRetry(ctx, s.dagRunStore, s.queueStore, dagSnapshot, latestStatus, exec.EnqueueRetryOptions{
		AutoRetry: true,
	})
	if err != nil {
		if errors.Is(err, exec.ErrRetryStaleLatest) {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(latestStatus.Name),
				tag.RunID(latestStatus.DAGRunID),
				slog.String("skip_reason", "stale_latest"),
			)
			return nil
		}
		return err
	}

	logger.Info(ctx, "Retry scanner ensured DAG-level retry is queued",
		tag.DAG(latestStatus.Name),
		tag.RunID(latestStatus.DAGRunID),
		slog.String("next_retry_at", decision.nextRetryAt.Format(time.RFC3339)),
		slog.Duration("computed_delay", decision.computedDelay),
	)
	return nil
}

func (s *RetryScanner) evaluateRetryDecision(
	_ context.Context,
	status *exec.DAGRunStatus,
	metadata dagRetryMetadata,
	now time.Time,
) retryDecision {
	if metadata.limit <= 0 {
		return retryDecision{reason: "retry_policy_missing"}
	}
	if status.AutoRetryCount >= metadata.limit {
		return retryDecision{reason: "retry_exhausted"}
	}

	referenceTime, ok := retryReferenceTime(status)
	if !ok {
		return retryDecision{reason: "missing_retry_reference_time"}
	}

	delay := dagRetryDelay(metadata.interval, metadata.backoff, metadata.maxInterval, status.AutoRetryCount)
	nextRetryAt := referenceTime.Add(delay)
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

func dagRetryDelay(interval time.Duration, backoff float64, maxInterval time.Duration, retryCount int) time.Duration {
	return core.CalculateBackoffInterval(interval, backoff, maxInterval, retryCount)
}

func retryReferenceTime(status *exec.DAGRunStatus) (time.Time, bool) {
	if status == nil {
		return time.Time{}, false
	}
	if finishedAt, ok := parseRFC3339(status.FinishedAt); ok {
		return finishedAt, true
	}
	if status.CreatedAt > 0 {
		return time.UnixMilli(status.CreatedAt).UTC(), true
	}
	if startedAt, ok := parseRFC3339(status.StartedAt); ok {
		return startedAt, true
	}
	return time.Time{}, false
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

func retryMetadataFromStatus(status *exec.DAGRunStatus) (dagRetryMetadata, bool) {
	if status == nil || status.ProcGroup == "" {
		return dagRetryMetadata{}, false
	}
	return dagRetryMetadata{
		limit:       status.AutoRetryLimit,
		interval:    status.AutoRetryInterval,
		backoff:     status.AutoRetryBackoff,
		maxInterval: status.AutoRetryMaxInterval,
	}, true
}

func retryMetadataFromDAG(dag *core.DAG) (dagRetryMetadata, bool) {
	if dag == nil || dag.RetryPolicy == nil {
		return dagRetryMetadata{}, false
	}
	return dagRetryMetadata{
		limit:       dag.RetryPolicy.Limit,
		interval:    dag.RetryPolicy.Interval,
		backoff:     dag.RetryPolicy.Backoff,
		maxInterval: dag.RetryPolicy.MaxInterval,
	}, true
}
