// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"log/slog"
	"math"
	"path/filepath"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
)

const (
	retryScanInterval               = 15 * time.Second
	failureFinalizationStaleTimeout = 5 * time.Minute
)

type retryAction int

const (
	retryActionSkip retryAction = iota
	retryActionEnqueue
	retryActionFinalize
)

type retryDecision struct {
	action        retryAction
	reason        string
	computedDelay time.Duration
	nextRetryAt   time.Time
}

type dagExecutioner interface {
	ExecuteDAG(
		ctx context.Context,
		dag *core.DAG,
		operation coordinatorv1.Operation,
		runID string,
		previousStatus *exec.DAGRunStatus,
		triggerType core.TriggerType,
		scheduleTime string,
	) error
}

// RetryScanner periodically discovers failed latest attempts and either
// enqueues a DAG-level retry or dispatches deferred terminal failure handling.
type RetryScanner struct {
	entryReader   EntryReader
	dagRunStore   exec.DAGRunStore
	queueStore    exec.QueueStore
	dagExecutor   dagExecutioner
	isSuspended   IsSuspendedFunc
	failureWindow time.Duration
	clock         Clock
}

func NewRetryScanner(
	entryReader EntryReader,
	dagRunStore exec.DAGRunStore,
	queueStore exec.QueueStore,
	dagExecutor dagExecutioner,
	isSuspended IsSuspendedFunc,
	failureWindow time.Duration,
	clock Clock,
) *RetryScanner {
	if clock == nil {
		clock = time.Now
	}
	if isSuspended == nil {
		isSuspended = func(context.Context, string) bool { return false }
	}
	return &RetryScanner{
		entryReader:   entryReader,
		dagRunStore:   dagRunStore,
		queueStore:    queueStore,
		dagExecutor:   dagExecutor,
		isSuspended:   isSuspended,
		failureWindow: failureWindow,
		clock:         clock,
	}
}

func (s *RetryScanner) Start(ctx context.Context) {
	if s == nil || s.failureWindow <= 0 {
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
	from := exec.NewUTC(now.Add(-s.failureWindow))

	failedRuns, err := s.dagRunStore.ListStatuses(
		ctx,
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
		exec.WithStatuses([]core.Status{core.Running, core.Queued}),
		exec.WithoutLimit(),
	)
	if err != nil {
		return err
	}

	currentDAGs := make(map[string]*core.DAG, len(s.entryReader.DAGs()))
	for _, dag := range s.entryReader.DAGs() {
		currentDAGs[dag.Name] = dag
	}

	activeByName := make(map[string][]*exec.DAGRunStatus)
	for _, st := range activeRuns {
		activeByName[st.Name] = append(activeByName[st.Name], st)
	}

	for _, listed := range failedRuns {
		if listed == nil {
			continue
		}
		if err := s.processFailedRun(ctx, listed, currentDAGs[listed.Name], activeByName[listed.Name], now); err != nil {
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
	if latestStatus.FailureFinalizedAt != "" {
		return nil
	}

	decision := s.evaluateRetryDecision(ctx, latestStatus, currentDAG, activeRuns, now)
	switch decision.action {
	case retryActionSkip:
		if decision.reason != "" {
			logger.Debug(ctx, "Retry scanner skipped DAG run",
				tag.DAG(latestStatus.Name),
				tag.RunID(latestStatus.DAGRunID),
				slog.String("skip_reason", decision.reason),
			)
		}
		return nil

	case retryActionEnqueue:
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

	case retryActionFinalize:
		dagSnapshot, err := attempt.ReadDAG(ctx)
		if err != nil {
			return err
		}
		return s.dispatchFailureFinalization(ctx, dagSnapshot, latestStatus, decision.reason, now)

	default:
		return nil
	}
}

func (s *RetryScanner) evaluateRetryDecision(
	ctx context.Context,
	status *exec.DAGRunStatus,
	currentDAG *core.DAG,
	activeRuns []*exec.DAGRunStatus,
	now time.Time,
) retryDecision {
	if status.FailureFinalizingAt != "" {
		if startedAt, ok := parseRFC3339(status.FailureFinalizingAt); ok && now.Sub(startedAt) < failureFinalizationStaleTimeout {
			return retryDecision{action: retryActionSkip, reason: "failure_finalization_in_progress"}
		}
	}

	if currentDAG == nil {
		return retryDecision{action: retryActionFinalize, reason: "dag_definition_missing"}
	}

	if currentDAG.RetryPolicy == nil {
		return retryDecision{action: retryActionFinalize, reason: "retry_policy_missing"}
	}

	if s.isSuspended(ctx, dagSuspendName(currentDAG)) {
		return retryDecision{action: retryActionSkip, reason: "suspended"}
	}

	if newerScheduledRunExists(status, activeRuns) {
		return retryDecision{action: retryActionFinalize, reason: "newer_run_exists"}
	}

	if status.RetryCount >= currentDAG.RetryPolicy.Limit {
		return retryDecision{action: retryActionFinalize, reason: "retry_exhausted"}
	}

	finishedAt, ok := parseRFC3339(status.FinishedAt)
	if !ok {
		return retryDecision{action: retryActionSkip, reason: "missing_finished_at"}
	}

	delay := dagRetryDelay(currentDAG.RetryPolicy, status.RetryCount)
	nextRetryAt := finishedAt.Add(delay)
	if now.Before(nextRetryAt) {
		return retryDecision{
			action:        retryActionSkip,
			reason:        "backoff_not_elapsed",
			computedDelay: delay,
			nextRetryAt:   nextRetryAt,
		}
	}

	return retryDecision{
		action:        retryActionEnqueue,
		computedDelay: delay,
		nextRetryAt:   nextRetryAt,
	}
}

func (s *RetryScanner) dispatchFailureFinalization(
	ctx context.Context,
	dag *core.DAG,
	status *exec.DAGRunStatus,
	reason string,
	now time.Time,
) error {
	finalizingAt := now.Format(time.RFC3339)
	updatedStatus, swapped, err := s.dagRunStore.CompareAndSwapLatestAttemptStatus(
		ctx,
		status.DAGRun(),
		status.AttemptID,
		core.Failed,
		func(latest *exec.DAGRunStatus) error {
			latest.FailureFinalizingAt = finalizingAt
			latest.FailureFinalizedAt = ""
			return nil
		},
	)
	if err != nil {
		return err
	}
	if !swapped {
		return nil
	}

	if err := s.dagExecutor.ExecuteDAG(
		ctx,
		dag,
		coordinatorv1.Operation_OPERATION_FINALIZE_FAILURE,
		status.DAGRunID,
		updatedStatus,
		status.TriggerType,
		status.ScheduleTime,
	); err != nil {
		_, _, _ = s.dagRunStore.CompareAndSwapLatestAttemptStatus(
			ctx,
			status.DAGRun(),
			status.AttemptID,
			core.Failed,
			func(latest *exec.DAGRunStatus) error {
				if latest.FailureFinalizingAt == finalizingAt {
					latest.FailureFinalizingAt = ""
				}
				return nil
			},
		)
		return err
	}

	logAttrs := []slog.Attr{
		tag.DAG(status.Name),
		tag.RunID(status.DAGRunID),
		slog.Int("retry_count", status.RetryCount),
		slog.String("skip_reason", reason),
	}
	if reason == "retry_exhausted" && dag.RetryPolicy != nil {
		logAttrs = append(logAttrs,
			slog.Int("limit", dag.RetryPolicy.Limit),
			slog.String("final_status", core.Failed.String()),
		)
		logger.Info(ctx, "Retry scanner finalized exhausted DAG run", logAttrs...)
		return nil
	}

	logger.Info(ctx, "Retry scanner dispatched terminal failure finalization", logAttrs...)
	return nil
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

func dagSuspendName(dag *core.DAG) string {
	if dag == nil {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(dag.Location), filepath.Ext(dag.Location))
	if base != "" && base != "." {
		return base
	}
	return dag.Name
}
