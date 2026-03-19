// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package slack

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/chatbridge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubNotificationDAGRunStore struct{}

func (s *stubNotificationDAGRunStore) CreateAttempt(context.Context, *core.DAG, time.Time, string, exec.NewDAGRunAttemptOptions) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) RecentAttempts(context.Context, string, int) []exec.DAGRunAttempt {
	return nil
}

func (s *stubNotificationDAGRunStore) LatestAttempt(context.Context, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) ListStatuses(context.Context, ...exec.ListDAGRunStatusesOption) ([]*exec.DAGRunStatus, error) {
	return nil, nil
}

func (s *stubNotificationDAGRunStore) CompareAndSwapLatestAttemptStatus(context.Context, exec.DAGRunRef, string, core.Status, func(*exec.DAGRunStatus) error) (*exec.DAGRunStatus, bool, error) {
	return nil, false, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) FindAttempt(context.Context, exec.DAGRunRef) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) FindSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) CreateSubAttempt(context.Context, exec.DAGRunRef, string) (exec.DAGRunAttempt, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) RemoveOldDAGRuns(context.Context, string, int, ...exec.RemoveOldDAGRunsOption) ([]string, error) {
	return nil, errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) RenameDAGRuns(context.Context, string, string) error {
	return errors.New("unexpected call")
}

func (s *stubNotificationDAGRunStore) RemoveDAGRun(context.Context, exec.DAGRunRef) error {
	return errors.New("unexpected call")
}

func TestDAGRunMonitor_RetriesOnlyUndeliveredSlackChannel(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{
		failChannels: map[string]int{"CFAIL": 2},
	}
	service := newFakeSlackAgentService("ai notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:             Config{SafeMode: true},
		agentAPI:        service,
		slackClient:     client,
		allowedChannels: map[string]struct{}{"COK": {}, "CFAIL": {}},
		logger:          logger,
	}
	monitor := NewDAGRunMonitor(nil, service, bot, logger)
	monitor.batcher = chatbridge.NewNotificationBatcher(10*time.Millisecond, 20*time.Millisecond, monitor.flushBatch)
	defer monitor.batcher.Stop()

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Error:     "boom",
	}

	require.True(t, monitor.notifyCompletion(context.Background(), status))
	require.Eventually(t, func() bool {
		return client.attemptsForChannel("COK") == 1 && client.attemptsForChannel("CFAIL") == 1
	}, time.Second, 10*time.Millisecond)
	assert.True(t, monitor.isSeen("COK", status))
	assert.False(t, monitor.isSeen("CFAIL", status))

	require.True(t, monitor.notifyCompletion(context.Background(), status))
	require.Eventually(t, func() bool {
		return client.attemptsForChannel("CFAIL") == 2
	}, time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, client.attemptsForChannel("COK"))

	service.mu.Lock()
	defer service.mu.Unlock()
	assert.Len(t, service.appendAttempts, 1)
}

func TestDAGRunMonitor_RunDrainsPendingSlackNotificationsWithoutLLM(t *testing.T) {
	t.Parallel()

	client := &fakeSlackClient{}
	service := newFakeSlackAgentService("ai notification")
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	bot := &Bot{
		cfg:             Config{SafeMode: true},
		agentAPI:        service,
		slackClient:     client,
		allowedChannels: map[string]struct{}{"D123": {}},
		logger:          logger,
	}
	monitor := NewDAGRunMonitor(&stubNotificationDAGRunStore{}, service, bot, logger)
	monitor.batcher = chatbridge.NewNotificationBatcher(time.Hour, time.Hour, monitor.flushBatch)

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
		Error:     "boom",
	}
	require.True(t, monitor.batcher.Enqueue("D123", status))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.Run(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for monitor shutdown drain")
	}

	service.mu.Lock()
	defer service.mu.Unlock()
	assert.Zero(t, service.generateCalls)
	require.Len(t, service.appendMessages, 1)
	assert.Contains(t, service.appendMessages[0].Content, "DAG `briefing` failed")
	assert.NotEqual(t, "ai notification", service.appendMessages[0].Content)
	assert.True(t, monitor.isSeen("D123", status))
}
