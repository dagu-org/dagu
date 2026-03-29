// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package eventfeed

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestWrappedAttemptWriteSuppressesDuplicateMeaningfulTransitions(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	attempt := &stubAttempt{
		id: "attempt-1",
		status: &exec.DAGRunStatus{
			Name:      "test-dag",
			DAGRunID:  "run-1",
			AttemptID: "attempt-1",
			Status:    core.Running,
		},
	}

	wrapped := &wrappedAttempt{
		next:     attempt,
		recorder: recorder,
		info: attemptContext{
			rootName:  "test-dag",
			rootRunID: "run-1",
		},
	}

	waitingStatus := exec.DAGRunStatus{
		Name:      "test-dag",
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Status:    core.Waiting,
		Nodes: []*exec.Node{
			{
				Step:              core.Step{Name: "approve"},
				Status:            core.NodeWaiting,
				StartedAt:         time.Now().UTC().Format(time.RFC3339),
				ApprovalIteration: 2,
			},
		},
	}

	require.NoError(t, wrapped.Write(context.Background(), waitingStatus))
	require.Len(t, recorder.entries(), 1)
	require.Equal(t, EventTypeWaiting, recorder.entries()[0].Type)
	require.Equal(t, "approve", recorder.entries()[0].StepName)
	require.NotNil(t, recorder.entries()[0].ApprovalIteration)
	require.Equal(t, 2, *recorder.entries()[0].ApprovalIteration)

	require.NoError(t, wrapped.Write(context.Background(), waitingStatus))
	require.Len(t, recorder.entries(), 1)
}

func TestDAGRunStoreCreateAttemptEmitsRootAndSubRunContext(t *testing.T) {
	t.Parallel()

	t.Run("RootAttempt", func(t *testing.T) {
		t.Parallel()

		recorder := &recordingRecorder{}
		store := &stubDAGRunStore{
			createAttempt: &stubAttempt{id: "attempt-root"},
		}
		wrapped := WrapDAGRunStore(store, recorder)

		dag := &core.DAG{Name: "root-dag"}
		attempt, err := wrapped.CreateAttempt(context.Background(), dag, time.Now().UTC(), "run-root", exec.NewDAGRunAttemptOptions{})
		require.NoError(t, err)

		err = attempt.Write(context.Background(), exec.DAGRunStatus{
			Name:      "root-dag",
			DAGRunID:  "run-root",
			AttemptID: "attempt-root",
			Status:    core.Failed,
			Error:     "boom",
			Nodes: []*exec.Node{
				{
					Step:       core.Step{Name: "build"},
					Status:     core.NodeFailed,
					FinishedAt: time.Now().UTC().Format(time.RFC3339),
					Error:      "boom",
				},
			},
		})
		require.NoError(t, err)

		entries := recorder.entries()
		require.Len(t, entries, 1)
		require.Equal(t, "root-dag", entries[0].DAGName)
		require.Equal(t, "run-root", entries[0].DAGRunID)
		require.Empty(t, entries[0].SubDAGRunID)
		require.Equal(t, "attempt-root", entries[0].AttemptID)
		require.Equal(t, "build", entries[0].StepName)
	})

	t.Run("SubAttempt", func(t *testing.T) {
		t.Parallel()

		recorder := &recordingRecorder{}
		store := &stubDAGRunStore{
			subAttempt: &stubAttempt{id: "attempt-sub"},
		}
		wrapped := WrapDAGRunStore(store, recorder)

		rootRef := exec.NewDAGRunRef("root-dag", "run-root")
		attempt, err := wrapped.CreateSubAttempt(context.Background(), rootRef, "sub-run-1")
		require.NoError(t, err)

		err = attempt.Write(context.Background(), exec.DAGRunStatus{
			Name:      "sub-dag",
			DAGRunID:  "sub-run-1",
			AttemptID: "attempt-sub",
			Status:    core.Waiting,
			Root:      rootRef,
			Nodes: []*exec.Node{
				{
					Step:      core.Step{Name: "review"},
					Status:    core.NodeWaiting,
					StartedAt: time.Now().UTC().Format(time.RFC3339),
				},
			},
		})
		require.NoError(t, err)

		entries := recorder.entries()
		require.Len(t, entries, 1)
		require.Equal(t, "root-dag", entries[0].DAGName)
		require.Equal(t, "run-root", entries[0].DAGRunID)
		require.Equal(t, "sub-run-1", entries[0].SubDAGRunID)
		require.Equal(t, "attempt-sub", entries[0].AttemptID)
		require.Equal(t, "review", entries[0].StepName)
	})
}

func TestDAGRunStoreCompareAndSwapLatestAttemptStatusEmitsEvent(t *testing.T) {
	t.Parallel()

	recorder := &recordingRecorder{}
	store := &stubDAGRunStore{
		casStatus: &exec.DAGRunStatus{
			Name:      "repair-dag",
			DAGRunID:  "run-1",
			AttemptID: "attempt-1",
			Status:    core.Running,
		},
		casSwapped: true,
	}
	wrapped := WrapDAGRunStore(store, recorder)

	status, swapped, err := wrapped.CompareAndSwapLatestAttemptStatus(
		context.Background(),
		exec.NewDAGRunRef("repair-dag", "run-1"),
		"attempt-1",
		core.Running,
		func(current *exec.DAGRunStatus) error {
			current.Status = core.Aborted
			current.Error = "repaired by coordinator"
			current.AttemptID = "attempt-1"
			current.Nodes = []*exec.Node{
				{
					Step:       core.Step{Name: "deploy"},
					Status:     core.NodeAborted,
					FinishedAt: time.Now().UTC().Format(time.RFC3339),
				},
			}
			return nil
		},
	)
	require.NoError(t, err)
	require.True(t, swapped)
	require.NotNil(t, status)

	entries := recorder.entries()
	require.Len(t, entries, 1)
	require.Equal(t, EventTypeAborted, entries[0].Type)
	require.Equal(t, "repair-dag", entries[0].DAGName)
	require.Equal(t, "run-1", entries[0].DAGRunID)
	require.Equal(t, "attempt-1", entries[0].AttemptID)
	require.Equal(t, "deploy", entries[0].StepName)
	require.Equal(t, core.Aborted.String(), entries[0].ResultingRunStatus)
}

func TestWrappedAttemptWriteSucceedsWhenRecorderFails(t *testing.T) {
	t.Parallel()

	attempt := &stubAttempt{
		id: "attempt-1",
		status: &exec.DAGRunStatus{
			Name:      "test-dag",
			DAGRunID:  "run-1",
			AttemptID: "attempt-1",
			Status:    core.Running,
		},
	}

	wrapped := &wrappedAttempt{
		next:     attempt,
		recorder: RecorderFunc(func(context.Context, Entry) error { return errors.New("append failed") }),
		info: attemptContext{
			rootName:  "test-dag",
			rootRunID: "run-1",
		},
	}

	newStatus := exec.DAGRunStatus{
		Name:      "test-dag",
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Status:    core.Failed,
		Error:     "boom",
	}
	require.NoError(t, wrapped.Write(context.Background(), newStatus))

	persisted, err := attempt.ReadStatus(context.Background())
	require.NoError(t, err)
	require.Equal(t, core.Failed, persisted.Status)
	require.Equal(t, "boom", persisted.Error)
}

func TestWrappedAttemptWriteSucceedsWhenRecorderTimesOut(t *testing.T) {
	t.Parallel()

	attempt := &stubAttempt{
		id: "attempt-1",
		status: &exec.DAGRunStatus{
			Name:      "test-dag",
			DAGRunID:  "run-1",
			AttemptID: "attempt-1",
			Status:    core.Running,
		},
	}

	service := New(&blockingStore{}, WithWriteTimeout(10*time.Millisecond))
	wrapped := &wrappedAttempt{
		next:     attempt,
		recorder: service,
		info: attemptContext{
			rootName:  "test-dag",
			rootRunID: "run-1",
		},
	}

	start := time.Now()
	err := wrapped.Write(context.Background(), exec.DAGRunStatus{
		Name:      "test-dag",
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Status:    core.Failed,
	})
	require.NoError(t, err)
	require.Less(t, time.Since(start), 250*time.Millisecond)

	persisted, readErr := attempt.ReadStatus(context.Background())
	require.NoError(t, readErr)
	require.Equal(t, core.Failed, persisted.Status)
}

type recordingRecorder struct {
	mu      sync.Mutex
	records []Entry
}

func (r *recordingRecorder) Record(_ context.Context, entry Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, entry)
	return nil
}

func (r *recordingRecorder) entries() []Entry {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Entry, len(r.records))
	copy(out, r.records)
	return out
}

type RecorderFunc func(context.Context, Entry) error

func (fn RecorderFunc) Record(ctx context.Context, entry Entry) error {
	return fn(ctx, entry)
}

type blockingStore struct{}

func (b *blockingStore) Append(ctx context.Context, _ *Entry) error {
	<-ctx.Done()
	return ctx.Err()
}

func (b *blockingStore) Query(context.Context, QueryFilter) (*QueryResult, error) {
	return &QueryResult{}, nil
}

func (b *blockingStore) Close() error {
	return nil
}

type stubAttempt struct {
	id     string
	status *exec.DAGRunStatus
	dag    *core.DAG
	hidden bool
}

func (a *stubAttempt) ID() string { return a.id }

func (a *stubAttempt) Open(context.Context) error { return nil }

func (a *stubAttempt) Write(_ context.Context, status exec.DAGRunStatus) error {
	copied := status
	a.status = &copied
	return nil
}

func (a *stubAttempt) Close(context.Context) error { return nil }

func (a *stubAttempt) ReadStatus(context.Context) (*exec.DAGRunStatus, error) {
	if a.status == nil {
		return nil, exec.ErrNoStatusData
	}
	copied := *a.status
	return &copied, nil
}

func (a *stubAttempt) ReadDAG(context.Context) (*core.DAG, error) { return a.dag, nil }

func (a *stubAttempt) SetDAG(dag *core.DAG) { a.dag = dag }

func (a *stubAttempt) Abort(context.Context) error { return nil }

func (a *stubAttempt) IsAborting(context.Context) (bool, error) { return false, nil }

func (a *stubAttempt) Hide(context.Context) error {
	a.hidden = true
	return nil
}

func (a *stubAttempt) Hidden() bool { return a.hidden }

func (a *stubAttempt) WriteOutputs(context.Context, *exec.DAGRunOutputs) error { return nil }

func (a *stubAttempt) ReadOutputs(context.Context) (*exec.DAGRunOutputs, error) { return nil, nil }

func (a *stubAttempt) WriteStepMessages(context.Context, string, []exec.LLMMessage) error { return nil }

func (a *stubAttempt) ReadStepMessages(context.Context, string) ([]exec.LLMMessage, error) {
	return nil, nil
}

func (a *stubAttempt) WorkDir() string { return "" }

type stubDAGRunStore struct {
	createAttempt exec.DAGRunAttempt
	subAttempt    exec.DAGRunAttempt
	latestAttempt exec.DAGRunAttempt
	recent        []exec.DAGRunAttempt
	casStatus     *exec.DAGRunStatus
	casSwapped    bool
}

func (s *stubDAGRunStore) CreateAttempt(context.Context, *core.DAG, time.Time, string, exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	return s.createAttempt, nil
}

func (s *stubDAGRunStore) RecentAttempts(context.Context, string, int) []exec.DAGRunAttempt {
	return s.recent
}

func (s *stubDAGRunStore) LatestAttempt(context.Context, string) (exec.DAGRunAttempt, error) {
	return s.latestAttempt, nil
}

func (s *stubDAGRunStore) ListStatuses(context.Context, ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return nil, nil
}

func (s *stubDAGRunStore) CompareAndSwapLatestAttemptStatus(
	_ context.Context,
	_ exec.DAGRunRef,
	_ string,
	_ core.Status,
	mutate func(*exec.DAGRunStatus) error,
) (*exec.DAGRunStatus, bool, error) {
	if !s.casSwapped {
		return s.casStatus, false, nil
	}
	current := *s.casStatus
	if err := mutate(&current); err != nil {
		return nil, false, err
	}
	s.casStatus = &current
	return &current, true, nil
}

func (s *stubDAGRunStore) FindAttempt(context.Context, exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	return s.createAttempt, nil
}

func (s *stubDAGRunStore) FindSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return s.subAttempt, nil
}

func (s *stubDAGRunStore) CreateSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return s.subAttempt, nil
}

func (s *stubDAGRunStore) RemoveOldDAGRuns(context.Context, string, int, ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, nil
}

func (s *stubDAGRunStore) RenameDAGRuns(context.Context, string, string) error { return nil }

func (s *stubDAGRunStore) RemoveDAGRun(context.Context, exec.DAGRunRef) error { return nil }
