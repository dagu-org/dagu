// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package slack

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	monitor := newDAGRunMonitorWithWindows(nil, "", service, bot, logger, 10*time.Millisecond, 20*time.Millisecond)
	stopMonitor := testutil.StartContextRunner(t, monitor)
	defer stopMonitor()

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-1",
		AttemptID: "attempt-1",
		Error:     "boom",
	}

	require.True(t, monitor.notifyCompletion(context.Background(), status))
	// Wait for the full flushPendingBatch cycle: PostMessage (increments attempts)
	// followed by markBatchDelivered (sets delivered/seen state).
	require.Eventually(t, func() bool {
		return client.attemptsForChannel("COK") == 1 &&
			client.attemptsForChannel("CFAIL") == 1 &&
			monitor.isSeen("COK", status) &&
			!monitor.isSeen("CFAIL", status)
	}, time.Second, 10*time.Millisecond)

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
	monitor := newDAGRunMonitorWithWindows(nil, "", service, bot, logger, time.Hour, time.Hour)

	status := &exec.DAGRunStatus{
		Name:      "briefing",
		Status:    core.Failed,
		DAGRunID:  "run-2",
		AttemptID: "attempt-2",
		Error:     "boom",
	}
	require.True(t, monitor.notifyCompletion(context.Background(), status))

	stopMonitor := testutil.StartContextRunner(t, monitor)
	stopMonitor()

	service.mu.Lock()
	defer service.mu.Unlock()
	assert.Zero(t, service.generateCalls)
	require.Len(t, service.appendMessages, 1)
	assert.Contains(t, service.appendMessages[0].Content, "DAG `briefing` failed")
	assert.NotEqual(t, "ai notification", service.appendMessages[0].Content)
	assert.True(t, monitor.isSeen("D123", status))
}
