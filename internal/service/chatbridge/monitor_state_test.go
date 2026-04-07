// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/dirlock"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/persis/fileeventstore"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/dagucloud/dagu/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationMonitor_BootstrapsFromCurrentHeadAndOnlyDeliversFutureEvents(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := fileeventstore.New(baseDir)
	require.NoError(t, err)
	service := eventstore.New(store)

	oldStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-old",
		AttemptID:  "attempt-old",
		Status:     core.Succeeded,
		FinishedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		oldStatus,
		nil,
	)))

	var (
		mu        sync.Mutex
		delivered []string
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, _ string, batch NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, event := range batch.Events {
				if event.DAGRun != nil {
					delivered = append(delivered, event.DAGRun.DAGRunID)
				}
			}
			return true
		},
	}

	cfg := newTestNotificationMonitorConfig()
	monitor := NewNotificationMonitor(service, filepath.Join(t.TempDir(), "state.json"), transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()
	require.Eventually(t, func() bool {
		monitor.stateMu.Lock()
		bootstrapped := monitor.state.Bootstrapped
		monitor.stateMu.Unlock()
		return monitor.ownsNotificationLock() && monitor.notificationSessionActive() && bootstrapped
	}, time.Second, 10*time.Millisecond)

	newStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-new",
		AttemptID:  "attempt-new",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		newStatus,
		nil,
	)))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) == 1 && delivered[0] == "run-new"
	}, time.Second, 10*time.Millisecond)

	assert.False(t, monitor.IsDelivered("dest-1", oldStatus))
	assert.True(t, monitor.IsDelivered("dest-1", newStatus))
}

func TestNotificationMonitor_RestartRequeuesPersistedPending(t *testing.T) {
	t.Parallel()

	stateFile := filepath.Join(t.TempDir(), "state.json")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := newTestNotificationMonitorConfig()

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Error:     "boom",
	}
	state := newNotificationMonitorState()
	state.Bootstrapped = true
	state.Destinations["dest-1"] = &notificationDestinationState{
		Pending: map[string]NotificationEvent{
			NotificationSeenKey(status): {
				Key:        NotificationSeenKey(status),
				DAGRun:     cloneNotificationStatus(status),
				ObservedAt: time.Now().UTC(),
			},
		},
		Delivered: make(map[string]time.Time),
	}
	require.NoError(t, newNotificationStateStore(stateFile).Save(context.Background(), state))

	var (
		mu    sync.Mutex
		calls int
	)
	secondTransport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, destination string, batch NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, "dest-1", destination)
			require.Len(t, batch.Events, 1)
			assert.Equal(t, "run-1", batch.Events[0].DAGRun.DAGRunID)
			calls++
			return true
		},
	}

	secondMonitor := NewNotificationMonitor(nil, stateFile, secondTransport, logger, cfg)
	stopMonitor := testutil.StartContextRunner(t, secondMonitor)
	defer stopMonitor()
	require.Eventually(t, func() bool {
		mu.Lock()
		called := calls
		mu.Unlock()
		return called >= 1 && secondMonitor.IsDelivered("dest-1", status)
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, calls, 1)
	assert.True(t, secondMonitor.IsDelivered("dest-1", status))
}

func TestNotificationStateStore_RejectsV1AndQuarantinesState(t *testing.T) {
	t.Parallel()

	stateFile := filepath.Join(t.TempDir(), "state.json")
	data, err := json.Marshal(struct {
		Version      int                           `json:"version"`
		Bootstrapped bool                          `json:"bootstrapped,omitempty"`
		SourceCursor eventstore.NotificationCursor `json:"source_cursor"`
		Destinations map[string]map[string]any     `json:"destinations,omitempty"`
	}{
		Version:      1,
		Bootstrapped: true,
		SourceCursor: eventstore.NotificationCursor{
			LastInboxFile:    "inbox-001.jsonl",
			CommittedOffsets: map[string]int64{"inbox-001.jsonl": 42},
		},
		Destinations: map[string]map[string]any{
			"dest-1": {
				"pending": map[string]any{
					"dag:run-1:attempt-1:failed": map[string]any{
						"key":        "dag:run-1:attempt-1:failed",
						"kind":       "dag_run",
						"type":       "dag.run.failed",
						"observedAt": "2026-04-01T00:00:00Z",
					},
				},
				"delivered": map[string]string{
					"dag:run-2:attempt-1:succeeded": time.Unix(1712016000, 0).UTC().Format(time.RFC3339),
				},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(stateFile, data, 0o600))

	store := newNotificationStateStore(stateFile)
	result := store.Load(context.Background())
	require.ErrorContains(t, result.Warning, "unsupported notification state version 1")
	require.True(t, result.Recovered)
	require.False(t, result.Missing)
	require.NotEmpty(t, result.QuarantinedPath)

	state := result.State
	require.Equal(t, notificationMonitorStateVersion, state.Version)
	require.False(t, state.Bootstrapped)
	require.Empty(t, state.SourceCursor.CommittedOffsets)
	require.Empty(t, state.Destinations)

	require.NoError(t, store.Save(context.Background(), state))
	var saved struct {
		Version int `json:"version"`
	}
	raw, err := os.ReadFile(stateFile)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, &saved))
	require.Equal(t, notificationMonitorStateVersion, saved.Version)
	_, err = os.Stat(result.QuarantinedPath)
	require.NoError(t, err)
}

func TestNotificationMonitor_StateLockAllowsSingleWriterAndTakeover(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := fileeventstore.New(baseDir)
	require.NoError(t, err)
	service := eventstore.New(store)

	stateFile := filepath.Join(t.TempDir(), "state.json")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var (
		mu         sync.Mutex
		deliveries = map[string][]string{
			"monitor-1": {},
			"monitor-2": {},
		}
	)
	newTransport := func(name string) *fakeNotificationTransport {
		return &fakeNotificationTransport{
			destinations: []string{"dest-1"},
			flushFn: func(_ context.Context, _ string, batch NotificationBatch, _ bool) bool {
				mu.Lock()
				defer mu.Unlock()
				for _, event := range batch.Events {
					if event.DAGRun != nil {
						deliveries[name] = append(deliveries[name], event.DAGRun.DAGRunID)
					}
				}
				return true
			},
		}
	}

	monitor1 := NewNotificationMonitor(service, stateFile, newTransport("monitor-1"), logger, newTestNotificationMonitorConfig())
	monitor2 := NewNotificationMonitor(service, stateFile, newTransport("monitor-2"), logger, newTestNotificationMonitorConfig())

	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	done1 := make(chan struct{})
	go func() {
		monitor1.Run(ctx1)
		close(done1)
	}()
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	done2 := make(chan struct{})
	go func() {
		monitor2.Run(ctx2)
		close(done2)
	}()
	defer func() {
		cancel1()
		cancel2()
		select {
		case <-done1:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for monitor-1 shutdown")
		}
		select {
		case <-done2:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for monitor-2 shutdown")
		}
	}()

	require.Eventually(t, func() bool {
		switch {
		case monitor1.ownsNotificationLock():
			monitor1.stateMu.Lock()
			bootstrapped := monitor1.state.Bootstrapped
			monitor1.stateMu.Unlock()
			return monitor1.notificationSessionActive() && bootstrapped
		case monitor2.ownsNotificationLock():
			monitor2.stateMu.Lock()
			bootstrapped := monitor2.state.Bootstrapped
			monitor2.stateMu.Unlock()
			return monitor2.notificationSessionActive() && bootstrapped
		default:
			return false
		}
	}, time.Second, 10*time.Millisecond)

	firstStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-first",
		AttemptID:  "attempt-first",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		firstStatus,
		nil,
	)))

	var firstOwner string
	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		total := len(deliveries["monitor-1"]) + len(deliveries["monitor-2"])
		if total != 1 {
			return false
		}
		switch {
		case len(deliveries["monitor-1"]) == 1:
			firstOwner = "monitor-1"
		case len(deliveries["monitor-2"]) == 1:
			firstOwner = "monitor-2"
		default:
			return false
		}
		return true
	}, time.Second, 10*time.Millisecond)

	switch firstOwner {
	case "monitor-1":
		cancel1()
		select {
		case <-done1:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for monitor-1 shutdown")
		}
		require.Eventually(t, func() bool {
			monitor2.stateMu.Lock()
			bootstrapped := monitor2.state.Bootstrapped
			monitor2.stateMu.Unlock()
			return monitor2.ownsNotificationLock() && monitor2.notificationSessionActive() && bootstrapped
		}, 2*time.Second, 10*time.Millisecond)
	case "monitor-2":
		cancel2()
		select {
		case <-done2:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for monitor-2 shutdown")
		}
		require.Eventually(t, func() bool {
			monitor1.stateMu.Lock()
			bootstrapped := monitor1.state.Bootstrapped
			monitor1.stateMu.Unlock()
			return monitor1.ownsNotificationLock() && monitor1.notificationSessionActive() && bootstrapped
		}, 2*time.Second, 10*time.Millisecond)
	default:
		t.Fatalf("first owner not determined: %q", firstOwner)
	}

	secondStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-second",
		AttemptID:  "attempt-second",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		secondStatus,
		nil,
	)))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		switch firstOwner {
		case "monitor-1":
			return slices.Contains(deliveries["monitor-2"], "run-second")
		case "monitor-2":
			return slices.Contains(deliveries["monitor-1"], "run-second")
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)
}

func TestNotificationMonitor_CorruptStateIsQuarantinedAndOnlyFutureEventsAreDelivered(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := fileeventstore.New(baseDir)
	require.NoError(t, err)
	service := eventstore.New(store)

	stateFile := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(stateFile, []byte("{not-json"), 0o600))

	oldStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-old",
		AttemptID:  "attempt-old",
		Status:     core.Succeeded,
		FinishedAt: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		oldStatus,
		nil,
	)))

	var (
		mu        sync.Mutex
		delivered []string
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, _ string, batch NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, event := range batch.Events {
				if event.DAGRun != nil {
					delivered = append(delivered, event.DAGRun.DAGRunID)
				}
			}
			return true
		},
	}

	monitor := NewNotificationMonitor(service, stateFile, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), newTestNotificationMonitorConfig())
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()

	require.Eventually(t, func() bool {
		matches, globErr := filepath.Glob(stateFile + ".corrupt.*")
		if globErr != nil {
			return false
		}
		return len(matches) == 1
	}, time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool {
		monitor.stateMu.Lock()
		defer monitor.stateMu.Unlock()
		return monitor.state.Bootstrapped
	}, time.Second, 10*time.Millisecond)

	newStatus := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-new",
		AttemptID:  "attempt-new",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		newStatus,
		nil,
	)))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) == 1 && delivered[0] == "run-new"
	}, time.Second, 10*time.Millisecond)

	assert.False(t, monitor.IsDelivered("dest-1", oldStatus))
	assert.True(t, monitor.IsDelivered("dest-1", newStatus))
}

func TestNotificationStateStore_LoadUnsupportedVersionQuarantinesState(t *testing.T) {
	t.Parallel()

	stateFile := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, os.WriteFile(stateFile, []byte(`{"version":99}`), 0o600))

	result := newNotificationStateStore(stateFile).Load(context.Background())
	require.Error(t, result.Warning)
	assert.True(t, result.Recovered)
	assert.NotEmpty(t, result.QuarantinedPath)
	assert.False(t, result.State.Bootstrapped)

	matches, err := filepath.Glob(stateFile + ".corrupt.*")
	require.NoError(t, err)
	require.Len(t, matches, 1)
}

func TestNotificationMonitor_SaveFailureDoesNotLoseUnreadEvents(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := fileeventstore.New(baseDir)
	require.NoError(t, err)
	service := eventstore.New(store)

	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "state.json")

	var (
		mu        sync.Mutex
		delivered []string
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, _ string, batch NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, event := range batch.Events {
				if event.DAGRun != nil {
					delivered = append(delivered, event.DAGRun.DAGRunID)
				}
			}
			return true
		},
	}

	monitor := NewNotificationMonitor(service, stateFile, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), newTestNotificationMonitorConfig())
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()

	require.Eventually(t, func() bool {
		monitor.stateMu.Lock()
		defer monitor.stateMu.Unlock()
		return monitor.state.Bootstrapped
	}, time.Second, 10*time.Millisecond)

	require.NoError(t, os.Chmod(stateDir, 0o500))
	defer func() {
		_ = os.Chmod(stateDir, 0o700)
	}()
	requireNotificationStateWriteFailure(t, monitor.stateStore, monitor.state)

	status := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-save-retry",
		AttemptID:  "attempt-save-retry",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		status,
		nil,
	)))

	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) > 0
	}, 150*time.Millisecond, 10*time.Millisecond)

	require.NoError(t, os.Chmod(stateDir, 0o700))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) == 1 && delivered[0] == "run-save-retry"
	}, time.Second, 10*time.Millisecond)

	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) > 1
	}, 150*time.Millisecond, 10*time.Millisecond)
	assert.True(t, monitor.IsDelivered("dest-1", status))
}

func TestNotificationMonitor_NotifyCompletionSaveFailureDoesNotMutateLiveState(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "state.json")
	monitor := NewNotificationMonitor(
		nil,
		stateFile,
		&fakeNotificationTransport{destinations: []string{"dest-1"}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		newTestNotificationMonitorConfig(),
	)
	monitor.lock = nil
	monitor.lockDir = ""

	require.NoError(t, os.Chmod(stateDir, 0o500))
	defer func() {
		_ = os.Chmod(stateDir, 0o700)
	}()
	requireNotificationStateWriteFailure(t, monitor.stateStore, monitor.state)

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		DAGRunID:  "run-save-fail",
		AttemptID: "attempt-save-fail",
		Status:    core.Failed,
		Error:     "boom",
	}
	require.False(t, monitor.NotifyCompletion(status))

	monitor.stateMu.Lock()
	defer monitor.stateMu.Unlock()
	destState := monitor.state.Destinations["dest-1"]
	require.Nil(t, destState)
}

func TestNotificationMonitor_MarkBatchDeliveredSaveFailureDoesNotMutateLiveState(t *testing.T) {
	t.Parallel()

	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "state.json")
	monitor := NewNotificationMonitor(
		nil,
		stateFile,
		&fakeNotificationTransport{destinations: []string{"dest-1"}},
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		newTestNotificationMonitorConfig(),
	)
	monitor.lock = nil
	monitor.lockDir = ""

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		DAGRunID:  "run-ack-save-fail",
		AttemptID: "attempt-ack-save-fail",
		Status:    core.Succeeded,
	}
	event := NotificationEvent{
		Key:        NotificationSeenKey(status),
		DAGRun:     cloneNotificationStatus(status),
		ObservedAt: time.Now().UTC(),
	}
	monitor.state.Destinations["dest-1"] = &notificationDestinationState{
		Pending: map[string]NotificationEvent{
			event.Key: event,
		},
		Delivered: make(map[string]time.Time),
	}

	require.NoError(t, os.Chmod(stateDir, 0o500))
	defer func() {
		_ = os.Chmod(stateDir, 0o700)
	}()
	requireNotificationStateWriteFailure(t, monitor.stateStore, monitor.state)

	monitor.markBatchDelivered(context.Background(), "dest-1", NotificationBatch{
		Class:  NotificationClassSuccessDigest,
		Events: []NotificationEvent{event},
	})

	monitor.stateMu.Lock()
	defer monitor.stateMu.Unlock()
	destState := monitor.state.Destinations["dest-1"]
	require.NotNil(t, destState)
	assert.Contains(t, destState.Pending, event.Key)
	assert.Empty(t, destState.Delivered)
}

func TestNotificationMonitor_RemovedDestinationsArePurgedOnStartup(t *testing.T) {
	t.Parallel()

	stateFile := filepath.Join(t.TempDir(), "state.json")
	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-removed",
		AttemptID: "attempt-removed",
		Error:     "boom",
	}
	state := newNotificationMonitorState()
	state.Bootstrapped = true
	state.Destinations["removed-dest"] = &notificationDestinationState{
		Pending: map[string]NotificationEvent{
			NotificationSeenKey(status): {
				Key:        NotificationSeenKey(status),
				DAGRun:     cloneNotificationStatus(status),
				ObservedAt: time.Now().UTC(),
			},
		},
		Delivered: map[string]time.Time{
			NotificationSeenKey(status): time.Now().UTC(),
		},
	}
	require.NoError(t, newNotificationStateStore(stateFile).Save(context.Background(), state))

	var (
		mu    sync.Mutex
		calls []string
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"keep-dest"},
		flushFn: func(_ context.Context, destination string, _ NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, destination)
			return true
		},
	}

	monitor := NewNotificationMonitor(nil, stateFile, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), newTestNotificationMonitorConfig())
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()

	require.Eventually(t, func() bool {
		monitor.stateMu.Lock()
		defer monitor.stateMu.Unlock()
		_, removedExists := monitor.state.Destinations["removed-dest"]
		_, keepExists := monitor.state.Destinations["keep-dest"]
		return !removedExists && keepExists
	}, time.Second, 10*time.Millisecond)

	result := newNotificationStateStore(stateFile).Load(context.Background())
	require.NoError(t, result.Warning)
	assert.NotContains(t, result.State.Destinations, "removed-dest")
	assert.Contains(t, result.State.Destinations, "keep-dest")

	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, calls)
}

func TestNotificationMonitor_LockTheftSelfFencesActiveOwner(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	store, err := fileeventstore.New(baseDir)
	require.NoError(t, err)
	service := eventstore.New(store)

	stateFile := filepath.Join(t.TempDir(), "state.json")

	var (
		mu        sync.Mutex
		delivered []string
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, _ string, batch NotificationBatch, _ bool) bool {
			mu.Lock()
			defer mu.Unlock()
			for _, event := range batch.Events {
				if event.DAGRun != nil {
					delivered = append(delivered, event.DAGRun.DAGRunID)
				}
			}
			return true
		},
	}

	monitor := NewNotificationMonitor(service, stateFile, transport, slog.New(slog.NewTextHandler(io.Discard, nil)), newTestNotificationMonitorConfig())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for monitor shutdown")
		}
	}()

	require.Eventually(t, func() bool {
		return monitor.ownsNotificationLock() && monitor.notificationSessionActive()
	}, time.Second, 10*time.Millisecond)

	lockDir := notificationStateLockDir(stateFile)
	lockTokenPath := filepath.Join(lockDir, ".dagu_lock", "owner")
	require.NoError(t, os.WriteFile(lockTokenPath, []byte("replacement-owner"), 0o600))
	require.Eventually(t, func() bool {
		return !monitor.ownsNotificationLock() && !monitor.notificationSessionActive()
	}, 2*time.Second, 10*time.Millisecond)

	require.NoError(t, dirlock.ForceUnlock(lockDir))
	replacement := dirlock.New(lockDir, &dirlock.LockOptions{
		StaleThreshold: time.Hour,
		RetryInterval:  10 * time.Millisecond,
	})
	require.NoError(t, replacement.TryLock())
	defer func() { _ = replacement.Unlock() }()

	status := &exec.DAGRunStatus{
		Name:       "briefing",
		DAGRunID:   "run-stolen-lock",
		AttemptID:  "attempt-stolen-lock",
		Status:     core.Succeeded,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunSucceeded,
		status,
		nil,
	)))

	require.Never(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) > 0
	}, 150*time.Millisecond, 10*time.Millisecond)
}
