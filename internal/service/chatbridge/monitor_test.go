// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationMonitor_ShutdownDrainRetriesInFlightBatchWithoutLLM(t *testing.T) {
	t.Parallel()

	type call struct {
		destination string
		allowLLM    bool
	}

	var (
		mu    sync.Mutex
		calls []call
	)
	firstCall := make(chan struct{}, 1)
	secondCall := make(chan struct{}, 1)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(ctx context.Context, destination string, _ NotificationBatch, allowLLM bool) bool {
			mu.Lock()
			calls = append(calls, call{destination: destination, allowLLM: allowLLM})
			callCount := len(calls)
			mu.Unlock()

			if callCount == 1 {
				firstCall <- struct{}{}
				<-ctx.Done()
				return false
			}
			secondCall <- struct{}{}
			return true
		},
	}
	cfg := DefaultNotificationMonitorConfig()
	cfg.UrgentWindow = 10 * time.Millisecond
	cfg.SuccessWindow = 10 * time.Millisecond
	cfg.FlushTimeout = time.Second
	cfg.PollInterval = time.Hour
	cfg.SeenEvictInterval = time.Hour

	monitor := NewNotificationMonitor(nil, "", transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Error:     "boom",
	}
	require.True(t, monitor.NotifyCompletion(status))

	select {
	case <-firstCall:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first flush attempt")
	}

	cancel()

	select {
	case <-secondCall:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for shutdown retry")
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for monitor shutdown")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calls, 2)
	assert.Equal(t, call{destination: "dest-1", allowLLM: true}, calls[0])
	assert.Equal(t, call{destination: "dest-1", allowLLM: false}, calls[1])
	assert.True(t, monitor.IsDelivered("dest-1", status))
}

func TestNotificationMonitor_BootstrapFailureDoesNotReplayFromZeroCursor(t *testing.T) {
	t.Parallel()

	store := &stubNotificationStore{failHead: true}
	service := eventstore.New(store)

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

	cfg := DefaultNotificationMonitorConfig()
	cfg.PollInterval = 10 * time.Millisecond
	cfg.SuccessWindow = 10 * time.Millisecond
	cfg.UrgentWindow = 10 * time.Millisecond
	cfg.SeenEvictInterval = time.Hour

	monitor := NewNotificationMonitor(service, "", transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

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

	require.Eventually(t, func() bool {
		headCalls, readCalls := store.stats()
		return headCalls > 0 && readCalls == 0
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Empty(t, delivered)
	mu.Unlock()

	store.setHeadFailure(false)
	require.Eventually(t, func() bool {
		monitor.stateMu.Lock()
		defer monitor.stateMu.Unlock()
		return monitor.state.Bootstrapped
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	assert.Empty(t, delivered)
	mu.Unlock()

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
}

func TestNotificationMonitor_ShutdownDrainFlushesPendingBatchWithoutLLM(t *testing.T) {
	t.Parallel()

	type call struct {
		destination string
		allowLLM    bool
	}

	var (
		mu    sync.Mutex
		calls []call
	)
	transport := &fakeNotificationTransport{
		destinations: []string{"dest-1"},
		flushFn: func(_ context.Context, destination string, _ NotificationBatch, allowLLM bool) bool {
			mu.Lock()
			defer mu.Unlock()
			calls = append(calls, call{destination: destination, allowLLM: allowLLM})
			return true
		},
	}
	cfg := DefaultNotificationMonitorConfig()
	cfg.UrgentWindow = time.Hour
	cfg.SuccessWindow = time.Hour
	cfg.FlushTimeout = time.Second
	cfg.PollInterval = time.Hour
	cfg.SeenEvictInterval = time.Hour

	monitor := NewNotificationMonitor(nil, "", transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Succeeded,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
	}
	require.True(t, monitor.NotifyCompletion(status))
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for monitor shutdown")
	}

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, calls, 1)
	assert.Equal(t, call{destination: "dest-1", allowLLM: false}, calls[0])
	assert.True(t, monitor.IsDelivered("dest-1", status))
}

func TestNotificationMonitor_PollSourceFiltersInterestedEventTypes(t *testing.T) {
	t.Parallel()

	store := &stubNotificationStore{}
	service := eventstore.New(store)
	transport := &fakeNotificationTransport{destinations: []string{"dest-1"}}

	cfg := DefaultNotificationMonitorConfig()
	cfg.UrgentWindow = 5 * time.Millisecond
	cfg.SuccessWindow = 5 * time.Millisecond
	cfg.InterestedEventTypes = []eventstore.EventType{eventstore.TypeDAGRunRunning}

	monitor := NewNotificationMonitor(service, "", transport, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg)
	monitor.initializeSession(context.Background())

	queued := &exec.DAGRunStatus{
		Name:      "briefing",
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Status:    core.Queued,
		QueuedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	running := *queued
	running.Status = core.Running
	running.StartedAt = time.Now().UTC().Format(time.RFC3339)

	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunQueued,
		queued,
		nil,
	)))
	require.NoError(t, service.Emit(context.Background(), eventstore.NewDAGRunEvent(
		eventstore.Source{Service: eventstore.SourceServiceServer, Instance: "test"},
		eventstore.TypeDAGRunRunning,
		&running,
		nil,
	)))

	monitor.pollSource(context.Background())

	ready := waitForReadyBatch(t, monitor.currentBatcher())
	require.Len(t, ready.Batch.Events, 1)
	assert.Equal(t, NotificationClassInformational, ready.Batch.Class)
	assert.Equal(t, eventstore.TypeDAGRunRunning, ready.Batch.Events[0].Type)
	require.NotNil(t, ready.Batch.Events[0].DAGRun)
	assert.Equal(t, core.Running, ready.Batch.Events[0].DAGRun.Status)
}
