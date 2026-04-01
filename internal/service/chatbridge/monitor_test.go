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

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNotificationTransport struct {
	destinations []string
	flushFn      func(context.Context, string, NotificationBatch, bool) bool
}

func (f *fakeNotificationTransport) NotificationDestinations() []string {
	return append([]string(nil), f.destinations...)
}

func (f *fakeNotificationTransport) FlushNotificationBatch(ctx context.Context, destination string, batch NotificationBatch, allowLLM bool) bool {
	if f.flushFn == nil {
		return true
	}
	return f.flushFn(ctx, destination, batch, allowLLM)
}

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
