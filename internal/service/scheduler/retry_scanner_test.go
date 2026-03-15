// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestRetryScannerEvaluateRetryDecision(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	baseDAG := &core.DAG{
		Name:        "retry-dag",
		RetryPolicy: &core.DAGRetryPolicy{Limit: 3, Interval: time.Minute, Backoff: 1.0, MaxInterval: 10 * time.Minute},
	}
	baseStatus := &exec.DAGRunStatus{
		Name:           "retry-dag",
		DAGRunID:       "run-1",
		AttemptID:      "att-1",
		Status:         core.Failed,
		AutoRetryCount: 0,
		FinishedAt:     now.Add(-2 * time.Minute).Format(time.RFC3339),
		ScheduleTime:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}

	tests := []struct {
		name        string
		status      *exec.DAGRunStatus
		dagSnapshot *core.DAG
		activeRuns  []*exec.DAGRunStatus
		suspended   bool
		enqueue     bool
		reason      string
		nextRetry   time.Time
		delay       time.Duration
	}{
		{
			name:        "SuspendedSkips",
			status:      cloneRetryStatus(baseStatus),
			dagSnapshot: baseDAG,
			suspended:   true,
			reason:      "suspended",
		},
		{
			name:        "NewerScheduledRunSkips",
			status:      cloneRetryStatus(baseStatus),
			dagSnapshot: baseDAG,
			activeRuns:  []*exec.DAGRunStatus{{Name: baseStatus.Name, DAGRunID: "run-2", ScheduleTime: now.Add(-5 * time.Minute).Format(time.RFC3339)}},
			reason:      "newer_run_exists",
		},
		{
			name:        "RetryExhaustedSkips",
			status:      withAutoRetryCount(baseStatus, 3),
			dagSnapshot: baseDAG,
			reason:      "retry_exhausted",
		},
		{
			name:        "MissingFinishedAtSkips",
			status:      withFinishedAt(baseStatus, ""),
			dagSnapshot: baseDAG,
			reason:      "missing_finished_at",
		},
		{
			name:        "BackoffNotElapsedSkips",
			status:      withFinishedAt(baseStatus, now.Add(-30*time.Second).Format(time.RFC3339)),
			dagSnapshot: baseDAG,
			reason:      "backoff_not_elapsed",
			nextRetry:   now.Add(30 * time.Second),
			delay:       time.Minute,
		},
		{
			name:        "EligibleFailureEnqueues",
			status:      cloneRetryStatus(baseStatus),
			dagSnapshot: baseDAG,
			enqueue:     true,
			nextRetry:   now.Add(-time.Minute),
			delay:       time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			scanner, err := NewRetryScanner(
				nil,
				nil,
				nil,
				func(context.Context, string) bool { return tt.suspended },
				24*time.Hour,
				func() time.Time { return now },
			)
			require.NoError(t, err)

			got := scanner.evaluateRetryDecision(context.Background(), tt.status, tt.dagSnapshot, tt.activeRuns, now)

			assert.Equal(t, tt.enqueue, got.enqueue)
			assert.Equal(t, tt.reason, got.reason)
			assert.Equal(t, tt.nextRetry, got.nextRetryAt)
			assert.Equal(t, tt.delay, got.computedDelay)
		})
	}
}

func TestNewRetryScanner(t *testing.T) {
	t.Parallel()

	scanner, err := NewRetryScanner(nil, nil, nil, nil, 0, time.Now)
	require.NoError(t, err)
	require.NotNil(t, scanner)
}

func TestDAGSuspendFlagName(t *testing.T) {
	t.Parallel()

	t.Run("UsesFilenameStem", func(t *testing.T) {
		t.Parallel()

		got := dagSuspendFlagName(&core.DAG{
			Name:     "logical-name",
			Location: "/tmp/example-dag.yaml",
		})

		assert.Equal(t, "example-dag", got)
	})

	t.Run("FallsBackToDAGNameWhenLocationMissing", func(t *testing.T) {
		t.Parallel()

		got := dagSuspendFlagName(&core.DAG{
			Name: "logical-name",
		})

		assert.Equal(t, "logical-name", got)
	})
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
		Name:           dag.Name,
		DAGRunID:       "run-1",
		AttemptID:      "att-1",
		Status:         core.Failed,
		AutoRetryCount: 1,
		FinishedAt:     now.Add(-3 * time.Minute).Format(time.RFC3339),
		ScheduleTime:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStore(dag, status)
	queueStore := &exec.MockQueueStore{}
	queueStore.On("Enqueue", mock.Anything, dag.ProcGroup(), exec.QueuePriorityLow, status.DAGRun()).
		Return(nil).
		Once()

	scanner, err := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		queueStore,
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)
	require.NoError(t, err)

	err = scanner.scan(context.Background())
	require.NoError(t, err)

	latest := store.mustStatus(status.DAGRun())
	assert.Equal(t, core.Queued, latest.Status)
	assert.Equal(t, core.TriggerTypeRetry, latest.TriggerType)
	assert.NotEmpty(t, latest.QueuedAt)
	assert.Equal(t, 2, latest.AutoRetryCount)
	assert.Len(t, store.listCalls, 2)
	assert.Empty(t, store.listCalls[0].ExactName)
	assert.False(t, store.listCalls[0].From.IsZero())
	assert.Empty(t, store.listCalls[1].ExactName)
	assert.False(t, store.listCalls[1].From.IsZero())

	queueStore.AssertExpectations(t)
}

func TestRetryScannerScanSkipsCrossMidnightNewerRun(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 15, 0, 10, 0, 0, time.UTC)
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
	failed := &exec.DAGRunStatus{
		Name:           dag.Name,
		DAGRunID:       "run-1",
		AttemptID:      "att-1",
		Status:         core.Failed,
		AutoRetryCount: 0,
		FinishedAt:     time.Date(2026, 3, 15, 0, 2, 0, 0, time.UTC).Format(time.RFC3339),
		ScheduleTime:   time.Date(2026, 3, 14, 23, 50, 0, 0, time.UTC).Format(time.RFC3339),
	}
	active := &exec.DAGRunStatus{
		Name:         dag.Name,
		DAGRunID:     "run-2",
		AttemptID:    "att-2",
		Status:       core.Running,
		ScheduleTime: time.Date(2026, 3, 14, 23, 59, 0, 0, time.UTC).Format(time.RFC3339),
	}

	store := newRetryScannerStore(dag, failed, active)
	scanner, err := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		&exec.MockQueueStore{},
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)
	require.NoError(t, err)

	err = scanner.scan(context.Background())
	require.NoError(t, err)

	latest := store.mustStatus(failed.DAGRun())
	assert.Equal(t, core.Failed, latest.Status)
	assert.Len(t, store.listCalls, 2)
	assert.False(t, store.listCalls[1].From.IsZero())
}

func TestRetryScannerScanUsesPersistedRetryPolicy(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 14, 14, 0, 0, 0, time.UTC)
	retryDAG := &core.DAG{
		Name:     "retry-dag",
		Location: "/tmp/retry-dag.yaml",
		RetryPolicy: &core.DAGRetryPolicy{
			Limit:       3,
			Interval:    time.Minute,
			Backoff:     1.0,
			MaxInterval: 10 * time.Minute,
		},
	}
	noRetryDAG := &core.DAG{Name: "plain-dag", Location: "/tmp/plain-dag.yaml"}
	retryStatus := &exec.DAGRunStatus{
		Name:           retryDAG.Name,
		DAGRunID:       "run-1",
		AttemptID:      "att-1",
		Status:         core.Failed,
		AutoRetryCount: 0,
		FinishedAt:     now.Add(-2 * time.Minute).Format(time.RFC3339),
		ScheduleTime:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	plainStatus := &exec.DAGRunStatus{
		Name:           noRetryDAG.Name,
		DAGRunID:       "run-2",
		AttemptID:      "att-2",
		Status:         core.Failed,
		AutoRetryCount: 0,
		FinishedAt:     now.Add(-2 * time.Minute).Format(time.RFC3339),
		ScheduleTime:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStoreWithEntries(
		retryScannerStoreEntry{dag: retryDAG, status: retryStatus},
		retryScannerStoreEntry{dag: noRetryDAG, status: plainStatus},
	)
	queueStore := &exec.MockQueueStore{}
	queueStore.On("Enqueue", mock.Anything, retryDAG.ProcGroup(), exec.QueuePriorityLow, retryStatus.DAGRun()).
		Return(nil).
		Once()

	scanner, err := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{retryDAG, noRetryDAG}},
		store,
		queueStore,
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)
	require.NoError(t, err)

	err = scanner.scan(context.Background())
	require.NoError(t, err)

	assert.Len(t, store.listCalls, 2)
	assert.Empty(t, store.listCalls[0].ExactName)
	assert.Empty(t, store.listCalls[1].ExactName)
	assert.Equal(t, 1, store.mustStatus(retryStatus.DAGRun()).AutoRetryCount)
	assert.Equal(t, core.Failed, store.mustStatus(plainStatus.DAGRun()).Status)
	queueStore.AssertExpectations(t)
}

func TestRetryScannerScanIsIdempotentForQueuedRun(t *testing.T) {
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
		Name:           dag.Name,
		DAGRunID:       "run-1",
		AttemptID:      "att-1",
		Status:         core.Failed,
		AutoRetryCount: 0,
		FinishedAt:     now.Add(-2 * time.Minute).Format(time.RFC3339),
		ScheduleTime:   now.Add(-10 * time.Minute).Format(time.RFC3339),
	}
	store := newRetryScannerStore(dag, status)
	queueStore := &exec.MockQueueStore{}
	queueStore.On("Enqueue", mock.Anything, dag.ProcGroup(), exec.QueuePriorityLow, status.DAGRun()).
		Return(nil).
		Once()

	scanner, err := NewRetryScanner(
		&retryScannerEntryReader{dags: []*core.DAG{dag}},
		store,
		queueStore,
		nil,
		24*time.Hour,
		func() time.Time { return now },
	)
	require.NoError(t, err)

	require.NoError(t, scanner.scan(context.Background()))
	require.NoError(t, scanner.scan(context.Background()))

	assert.Equal(t, core.Queued, store.mustStatus(status.DAGRun()).Status)
	queueStore.AssertExpectations(t)
}

type retryScannerEntryReader struct {
	dags []*core.DAG
}

func (r *retryScannerEntryReader) Init(context.Context) error { return nil }
func (r *retryScannerEntryReader) Start(context.Context)      {}
func (r *retryScannerEntryReader) Stop()                      {}
func (r *retryScannerEntryReader) DAGs() []*core.DAG          { return r.dags }

type retryScannerStore struct {
	attempts  map[string]*retryScannerAttempt
	listCalls []exec.ListDAGRunStatusesOptions
}

type retryScannerStoreEntry struct {
	dag    *core.DAG
	status *exec.DAGRunStatus
}

func newRetryScannerStore(dag *core.DAG, statuses ...*exec.DAGRunStatus) *retryScannerStore {
	entries := make([]retryScannerStoreEntry, 0, len(statuses))
	for _, status := range statuses {
		if status == nil {
			continue
		}
		entries = append(entries, retryScannerStoreEntry{dag: dag, status: status})
	}
	return newRetryScannerStoreWithEntries(entries...)
}

func newRetryScannerStoreWithEntries(entries ...retryScannerStoreEntry) *retryScannerStore {
	attempts := make(map[string]*retryScannerAttempt, len(entries))
	for _, entry := range entries {
		if entry.status == nil {
			continue
		}
		attempts[entry.status.DAGRun().String()] = &retryScannerAttempt{
			id:     entry.status.AttemptID,
			status: cloneRetryStatus(entry.status),
			dag:    entry.dag,
		}
	}
	return &retryScannerStore{attempts: attempts}
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

	var ret []*exec.DAGRunStatus
	for _, attempt := range s.attempts {
		status := attempt.status
		if status == nil {
			continue
		}
		if cfg.ExactName != "" && status.Name != cfg.ExactName {
			continue
		}
		if len(cfg.Statuses) > 0 && !containsStatus(cfg.Statuses, status.Status) {
			continue
		}
		ret = append(ret, cloneRetryStatus(status))
	}
	return ret, nil
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

func containsStatus(statuses []core.Status, want core.Status) bool {
	return slices.Contains(statuses, want)
}

func withAutoRetryCount(status *exec.DAGRunStatus, retryCount int) *exec.DAGRunStatus {
	cloned := cloneRetryStatus(status)
	cloned.AutoRetryCount = retryCount
	return cloned
}

func withFinishedAt(status *exec.DAGRunStatus, finishedAt string) *exec.DAGRunStatus {
	cloned := cloneRetryStatus(status)
	cloned.FinishedAt = finishedAt
	return cloned
}
