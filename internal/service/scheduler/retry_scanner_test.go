// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	coordinatorv1 "github.com/dagu-org/dagu/proto/coordinator/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRetryScannerEvaluateRetryDecision(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	basePolicy := &core.DAGRetryPolicy{
		Limit:       3,
		Interval:    time.Minute,
		Backoff:     1.0,
		MaxInterval: 10 * time.Minute,
	}
	baseDAG := &core.DAG{
		Name:        "retry-dag",
		RetryPolicy: basePolicy,
	}
	baseStatus := &exec.DAGRunStatus{
		Name:         "retry-dag",
		DAGRunID:     "run-1",
		AttemptID:    "att-1",
		Status:       core.Failed,
		RetryCount:   0,
		FinishedAt:   now.Add(-2 * time.Minute).Format(time.RFC3339),
		ScheduleTime: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	newScanner := func(isSuspended IsSuspendedFunc) *RetryScanner {
		return NewRetryScanner(nil, nil, nil, nil, isSuspended, 24*time.Hour, func() time.Time {
			return now
		})
	}

	tests := []struct {
		name       string
		scanner    *RetryScanner
		status     *exec.DAGRunStatus
		currentDAG *core.DAG
		activeRuns []*exec.DAGRunStatus
		wantAction retryAction
		wantReason string
		wantNextAt time.Time
		wantDelay  time.Duration
	}{
		{
			name:       "FailureFinalizationInProgressSkips",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(withFailureFinalizingAt(baseStatus, now.Add(-time.Minute).Format(time.RFC3339))),
			currentDAG: baseDAG,
			wantAction: retryActionSkip,
			wantReason: "failure_finalization_in_progress",
		},
		{
			name:       "MissingDAGFinalizes",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(baseStatus),
			wantAction: retryActionFinalize,
			wantReason: "dag_definition_missing",
		},
		{
			name:       "MissingRetryPolicyFinalizes",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(baseStatus),
			currentDAG: &core.DAG{Name: "retry-dag"},
			wantAction: retryActionFinalize,
			wantReason: "retry_policy_missing",
		},
		{
			name:       "SuspendedSkips",
			scanner:    newScanner(func(context.Context, string) bool { return true }),
			status:     cloneRetryStatus(baseStatus),
			currentDAG: baseDAG,
			wantAction: retryActionSkip,
			wantReason: "suspended",
		},
		{
			name:       "NewerScheduledRunFinalizes",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(baseStatus),
			currentDAG: baseDAG,
			activeRuns: []*exec.DAGRunStatus{
				{
					Name:         "retry-dag",
					DAGRunID:     "run-2",
					Status:       core.Running,
					ScheduleTime: now.Add(-5 * time.Minute).Format(time.RFC3339),
				},
			},
			wantAction: retryActionFinalize,
			wantReason: "newer_run_exists",
		},
		{
			name:       "RetryExhaustedFinalizes",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(withRetryCount(baseStatus, 3)),
			currentDAG: baseDAG,
			wantAction: retryActionFinalize,
			wantReason: "retry_exhausted",
		},
		{
			name:       "MissingFinishedAtSkips",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(withFinishedAt(baseStatus, "")),
			currentDAG: baseDAG,
			wantAction: retryActionSkip,
			wantReason: "missing_finished_at",
		},
		{
			name:       "BackoffNotElapsedSkips",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(withFinishedAt(baseStatus, now.Add(-30*time.Second).Format(time.RFC3339))),
			currentDAG: baseDAG,
			wantAction: retryActionSkip,
			wantReason: "backoff_not_elapsed",
			wantDelay:  time.Minute,
			wantNextAt: now.Add(30 * time.Second),
		},
		{
			name:       "EligibleFailureEnqueues",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(baseStatus),
			currentDAG: baseDAG,
			wantAction: retryActionEnqueue,
			wantDelay:  time.Minute,
			wantNextAt: now.Add(-time.Minute),
		},
		{
			name:       "StaleFailureFinalizationRetries",
			scanner:    newScanner(nil),
			status:     cloneRetryStatus(withFailureFinalizingAt(baseStatus, now.Add(-10*time.Minute).Format(time.RFC3339))),
			currentDAG: baseDAG,
			wantAction: retryActionEnqueue,
			wantDelay:  time.Minute,
			wantNextAt: now.Add(-time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.scanner.evaluateRetryDecision(context.Background(), tt.status, tt.currentDAG, tt.activeRuns, now)

			assert.Equal(t, tt.wantAction, got.action)
			assert.Equal(t, tt.wantReason, got.reason)
			assert.Equal(t, tt.wantDelay, got.computedDelay)
			assert.Equal(t, tt.wantNextAt, got.nextRetryAt)
		})
	}
}

func TestRetryScannerScanEnqueuesRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	dag := &core.DAG{
		Name:     "retry-dag",
		Location: "/tmp/retry-dag.yaml",
		RetryPolicy: &core.DAGRetryPolicy{
			Limit:       3,
			Interval:    time.Minute,
			Backoff:     1.0,
			MaxInterval: 10 * time.Minute,
		},
	}
	status := &exec.DAGRunStatus{
		Name:         dag.Name,
		DAGRunID:     "run-1",
		AttemptID:    "att-1",
		Status:       core.Failed,
		RetryCount:   1,
		FinishedAt:   now.Add(-3 * time.Minute).Format(time.RFC3339),
		ScheduleTime: now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStore(status, dag)
	queueStore := &exec.MockQueueStore{}
	queueStore.On("Enqueue", mock.Anything, dag.ProcGroup(), exec.QueuePriorityLow, status.DAGRun()).
		Return(nil).
		Once()

	scanner := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		queueStore,
		nil,
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)

	err := scanner.scan(context.Background())
	require.NoError(t, err)

	latest := store.mustStatus(status.DAGRun())
	assert.Equal(t, core.Queued, latest.Status)
	assert.Equal(t, core.TriggerTypeRetry, latest.TriggerType)
	assert.NotEmpty(t, latest.QueuedAt)
	assert.Equal(t, 1, latest.RetryCount)
	assert.Len(t, store.listCalls, 2)
	assert.False(t, store.listCalls[0].From.IsZero())
	assert.True(t, store.listCalls[1].From.IsZero(), "active run scan should not be bounded by retry_failure_window")

	queueStore.AssertExpectations(t)
}

func TestRetryScannerScanFinalizesTerminalFailure(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	dag := &core.DAG{
		Name:     "retry-dag",
		Location: "/tmp/retry-dag.yaml",
		RetryPolicy: &core.DAGRetryPolicy{
			Limit:       1,
			Interval:    time.Minute,
			Backoff:     1.0,
			MaxInterval: 10 * time.Minute,
		},
	}
	status := &exec.DAGRunStatus{
		Name:         dag.Name,
		DAGRunID:     "run-1",
		AttemptID:    "att-1",
		Status:       core.Failed,
		RetryCount:   1,
		TriggerType:  core.TriggerTypeScheduler,
		FinishedAt:   now.Add(-10 * time.Minute).Format(time.RFC3339),
		ScheduleTime: now.Add(-20 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStore(status, dag)
	executor := &stubRetryScannerExecutor{}
	scanner := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		&exec.MockQueueStore{},
		executor,
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)

	err := scanner.scan(context.Background())
	require.NoError(t, err)

	latest := store.mustStatus(status.DAGRun())
	assert.Equal(t, core.Failed, latest.Status)
	assert.NotEmpty(t, latest.FailureFinalizingAt)
	assert.Empty(t, latest.FailureFinalizedAt)
	require.Len(t, executor.calls, 1)
	assert.Equal(t, coordinatorv1.Operation_OPERATION_FINALIZE_FAILURE, executor.calls[0].operation)
	assert.Equal(t, status.DAGRunID, executor.calls[0].runID)
	require.NotNil(t, executor.calls[0].previousStatus)
	assert.Equal(t, latest.AttemptID, executor.calls[0].previousStatus.AttemptID)
	assert.Equal(t, latest.FailureFinalizingAt, executor.calls[0].previousStatus.FailureFinalizingAt)
}

func TestRetryScannerDispatchFailureFinalizationRollsBackMarkerOnError(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	dag := &core.DAG{
		Name:     "retry-dag",
		Location: "/tmp/retry-dag.yaml",
		RetryPolicy: &core.DAGRetryPolicy{
			Limit:       1,
			Interval:    time.Minute,
			Backoff:     1.0,
			MaxInterval: 10 * time.Minute,
		},
	}
	status := &exec.DAGRunStatus{
		Name:         dag.Name,
		DAGRunID:     "run-1",
		AttemptID:    "att-1",
		Status:       core.Failed,
		RetryCount:   1,
		TriggerType:  core.TriggerTypeScheduler,
		FinishedAt:   now.Add(-10 * time.Minute).Format(time.RFC3339),
		ScheduleTime: now.Add(-20 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStore(status, dag)
	scanner := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		&exec.MockQueueStore{},
		&stubRetryScannerExecutor{err: errors.New("dispatch failed")},
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)

	err := scanner.dispatchFailureFinalization(context.Background(), dag, cloneRetryStatus(status), "retry_exhausted", now)
	require.ErrorContains(t, err, "dispatch failed")

	latest := store.mustStatus(status.DAGRun())
	assert.Empty(t, latest.FailureFinalizingAt)
	assert.Empty(t, latest.FailureFinalizedAt)
	assert.Equal(t, core.Failed, latest.Status)
}

type retryScannerEntryReader struct {
	dags []*core.DAG
}

func (r *retryScannerEntryReader) Init(context.Context) error { return nil }
func (r *retryScannerEntryReader) Start(context.Context)      {}
func (r *retryScannerEntryReader) Stop()                      {}
func (r *retryScannerEntryReader) DAGs() []*core.DAG          { return r.dags }

type retryScannerStore struct {
	failed    []*exec.DAGRunStatus
	active    []*exec.DAGRunStatus
	attempts  map[string]*retryScannerAttempt
	listCalls []exec.ListDAGRunStatusesOptions
}

func newRetryScannerStore(status *exec.DAGRunStatus, dag *core.DAG) *retryScannerStore {
	ref := status.DAGRun()
	return &retryScannerStore{
		failed: []*exec.DAGRunStatus{cloneRetryStatus(status)},
		active: nil,
		attempts: map[string]*retryScannerAttempt{
			ref.String(): {
				id:     status.AttemptID,
				status: cloneRetryStatus(status),
				dag:    dag,
			},
		},
	}
}

func (s *retryScannerStore) CreateAttempt(context.Context, *core.DAG, time.Time, string, exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected CreateAttempt call")
}

func (s *retryScannerStore) RecentAttempts(context.Context, string, int) []exec.DAGRunAttempt {
	return nil
}

func (s *retryScannerStore) LatestAttempt(context.Context, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected LatestAttempt call")
}

func (s *retryScannerStore) ListStatuses(_ context.Context, opts ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	var cfg exec.ListDAGRunStatusesOptions
	for _, opt := range opts {
		opt(&cfg)
	}
	s.listCalls = append(s.listCalls, cfg)

	if len(cfg.Statuses) == 1 && cfg.Statuses[0] == core.Failed {
		return cloneRetryStatuses(s.failed), nil
	}
	if len(cfg.Statuses) == 2 && containsStatuses(cfg.Statuses, core.Running, core.Queued) {
		return cloneRetryStatuses(s.active), nil
	}
	return nil, nil
}

func (s *retryScannerStore) CompareAndSwapLatestAttemptStatus(
	_ context.Context,
	dagRun exec.DAGRunRef,
	expectedAttemptID string,
	expectedStatus core.Status,
	mutate func(*exec.DAGRunStatus) error,
) (*exec.DAGRunStatus, bool, error) {
	attempt, ok := s.attempts[dagRun.String()]
	if !ok {
		return nil, false, nil
	}
	current := cloneRetryStatus(attempt.status)
	if current.AttemptID != expectedAttemptID || current.Status != expectedStatus {
		return current, false, nil
	}
	if err := mutate(current); err != nil {
		return nil, false, err
	}
	attempt.status = cloneRetryStatus(current)
	return cloneRetryStatus(attempt.status), true, nil
}

func (s *retryScannerStore) FindAttempt(_ context.Context, dagRun exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	attempt, ok := s.attempts[dagRun.String()]
	if !ok {
		return nil, exec.ErrDAGRunIDNotFound
	}
	return attempt, nil
}

func (s *retryScannerStore) FindSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected FindSubAttempt call")
}

func (s *retryScannerStore) CreateSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected CreateSubAttempt call")
}

func (s *retryScannerStore) RemoveOldDAGRuns(context.Context, string, int, ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, nil
}

func (s *retryScannerStore) RenameDAGRuns(context.Context, string, string) error { return nil }
func (s *retryScannerStore) RemoveDAGRun(context.Context, exec.DAGRunRef) error  { return nil }

func (s *retryScannerStore) mustStatus(ref exec.DAGRunRef) *exec.DAGRunStatus {
	attempt, ok := s.attempts[ref.String()]
	if !ok {
		return nil
	}
	return cloneRetryStatus(attempt.status)
}

type retryScannerAttempt struct {
	id     string
	status *exec.DAGRunStatus
	dag    *core.DAG
}

func (a *retryScannerAttempt) ID() string { return a.id }
func (a *retryScannerAttempt) Open(context.Context) error {
	return errors.New("unexpected Open call")
}
func (a *retryScannerAttempt) Write(context.Context, exec.DAGRunStatus) error {
	return errors.New("unexpected Write call")
}
func (a *retryScannerAttempt) Close(context.Context) error { return nil }
func (a *retryScannerAttempt) ReadStatus(context.Context) (*exec.DAGRunStatus, error) {
	return cloneRetryStatus(a.status), nil
}
func (a *retryScannerAttempt) ReadDAG(context.Context) (*core.DAG, error) { return a.dag, nil }
func (a *retryScannerAttempt) SetDAG(*core.DAG)                           {}
func (a *retryScannerAttempt) Abort(context.Context) error                { return nil }
func (a *retryScannerAttempt) IsAborting(context.Context) (bool, error)   { return false, nil }
func (a *retryScannerAttempt) Hide(context.Context) error                 { return nil }
func (a *retryScannerAttempt) Hidden() bool                               { return false }
func (a *retryScannerAttempt) WriteOutputs(context.Context, *exec.DAGRunOutputs) error {
	return nil
}
func (a *retryScannerAttempt) ReadOutputs(context.Context) (*exec.DAGRunOutputs, error) {
	return nil, nil
}
func (a *retryScannerAttempt) WriteStepMessages(context.Context, string, []exec.LLMMessage) error {
	return nil
}
func (a *retryScannerAttempt) ReadStepMessages(context.Context, string) ([]exec.LLMMessage, error) {
	return nil, nil
}
func (a *retryScannerAttempt) WorkDir() string { return "" }

type stubRetryScannerExecutor struct {
	calls []retryScannerExecutorCall
	err   error
}

type retryScannerExecutorCall struct {
	dag            *core.DAG
	operation      coordinatorv1.Operation
	runID          string
	previousStatus *exec.DAGRunStatus
	triggerType    core.TriggerType
	scheduleTime   string
}

func (e *stubRetryScannerExecutor) ExecuteDAG(
	_ context.Context,
	dag *core.DAG,
	operation coordinatorv1.Operation,
	runID string,
	previousStatus *exec.DAGRunStatus,
	triggerType core.TriggerType,
	scheduleTime string,
) error {
	e.calls = append(e.calls, retryScannerExecutorCall{
		dag:            dag,
		operation:      operation,
		runID:          runID,
		previousStatus: cloneRetryStatus(previousStatus),
		triggerType:    triggerType,
		scheduleTime:   scheduleTime,
	})
	return e.err
}

func cloneRetryStatuses(statuses []*exec.DAGRunStatus) []*exec.DAGRunStatus {
	ret := make([]*exec.DAGRunStatus, 0, len(statuses))
	for _, st := range statuses {
		ret = append(ret, cloneRetryStatus(st))
	}
	return ret
}

func cloneRetryStatus(status *exec.DAGRunStatus) *exec.DAGRunStatus {
	if status == nil {
		return nil
	}
	cloned := *status
	if status.Nodes != nil {
		cloned.Nodes = append([]*exec.Node(nil), status.Nodes...)
	}
	return &cloned
}

func containsStatuses(statuses []core.Status, expected ...core.Status) bool {
	if len(statuses) != len(expected) {
		return false
	}
	seen := make(map[core.Status]bool, len(statuses))
	for _, st := range statuses {
		seen[st] = true
	}
	for _, st := range expected {
		if !seen[st] {
			return false
		}
	}
	return true
}

func withRetryCount(status *exec.DAGRunStatus, retryCount int) *exec.DAGRunStatus {
	cloned := cloneRetryStatus(status)
	cloned.RetryCount = retryCount
	return cloned
}

func withFinishedAt(status *exec.DAGRunStatus, finishedAt string) *exec.DAGRunStatus {
	cloned := cloneRetryStatus(status)
	cloned.FinishedAt = finishedAt
	return cloned
}

func withFailureFinalizingAt(status *exec.DAGRunStatus, finalizingAt string) *exec.DAGRunStatus {
	cloned := cloneRetryStatus(status)
	cloned.FailureFinalizingAt = finalizingAt
	return cloned
}
