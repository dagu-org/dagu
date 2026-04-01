// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/fileeventstore"
	"github.com/dagu-org/dagu/internal/service/eventstore"
	"github.com/dagu-org/dagu/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestNotificationMonitorConfig() NotificationMonitorConfig {
	cfg := DefaultNotificationMonitorConfig()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.SuccessWindow = 10 * time.Millisecond
	cfg.UrgentWindow = 10 * time.Millisecond
	cfg.SeenEvictInterval = time.Hour
	return cfg
}

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
				if event.Status != nil {
					delivered = append(delivered, event.Status.DAGRunID)
				}
			}
			return true
		},
	}

	cfg := newTestNotificationMonitorConfig()
	monitor := NewNotificationMonitor(service, filepath.Join(t.TempDir(), "state.json"), transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()
	time.Sleep(50 * time.Millisecond)

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

	firstTransport := &fakeNotificationTransport{destinations: []string{"dest-1"}}
	firstMonitor := NewNotificationMonitor(nil, stateFile, firstTransport, logger, cfg)

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Error:     "boom",
	}
	require.True(t, firstMonitor.NotifyCompletion(status))

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
			assert.Equal(t, "run-1", batch.Events[0].Status.DAGRunID)
			calls++
			return true
		},
	}

	secondMonitor := NewNotificationMonitor(nil, stateFile, secondTransport, logger, cfg)
	stopMonitor := testutil.StartContextRunner(t, secondMonitor)
	stopMonitor()

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 1, calls)
	assert.True(t, secondMonitor.IsDelivered("dest-1", status))
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
				if event.Status != nil {
					delivered = append(delivered, event.Status.DAGRunID)
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
				if event.Status != nil {
					delivered = append(delivered, event.Status.DAGRunID)
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

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Empty(t, delivered)
	mu.Unlock()

	require.NoError(t, os.Chmod(stateDir, 0o700))

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(delivered) == 1 && delivered[0] == "run-save-retry"
	}, time.Second, 10*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	assert.Len(t, delivered, 1)
	mu.Unlock()
	assert.True(t, monitor.IsDelivered("dest-1", status))
}
