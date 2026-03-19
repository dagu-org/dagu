// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package chatbridge

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNotificationSeenKeyIncludesStatus(t *testing.T) {
	t.Parallel()

	waiting := &exec.DAGRunStatus{DAGRunID: "run-1", AttemptID: "attempt-1", Status: core.Waiting}
	succeeded := &exec.DAGRunStatus{DAGRunID: "run-1", AttemptID: "attempt-1", Status: core.Succeeded}

	assert.NotEqual(t, NotificationSeenKey(waiting), NotificationSeenKey(succeeded))
}

func TestNotificationBatcher_SuccessBurstFlushesSingleDigest(t *testing.T) {
	t.Parallel()

	type flushedBatch struct {
		destination string
		batch       NotificationBatch
	}

	flushCh := make(chan flushedBatch, 1)
	batcher := NewNotificationBatcher(10*time.Millisecond, 20*time.Millisecond, func(destination string, batch NotificationBatch) {
		flushCh <- flushedBatch{destination: destination, batch: batch}
	})
	defer batcher.Stop()

	require.True(t, batcher.Enqueue("dest-1", &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-1", AttemptID: "a1", Status: core.Succeeded}))
	require.True(t, batcher.Enqueue("dest-1", &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-2", AttemptID: "a2", Status: core.Succeeded}))
	require.True(t, batcher.Enqueue("dest-1", &exec.DAGRunStatus{Name: "sync", DAGRunID: "run-3", AttemptID: "a3", Status: core.PartiallySucceeded}))

	select {
	case flushed := <-flushCh:
		assert.Equal(t, "dest-1", flushed.destination)
		assert.Equal(t, NotificationClassSuccessDigest, flushed.batch.Class)
		assert.Len(t, flushed.batch.Events, 3)
		text := FormatNotificationBatch(flushed.batch)
		assert.Contains(t, text, "DAG completion digest")
		assert.Contains(t, text, "briefing: succeeded x2")
		assert.Contains(t, text, "sync: partially_succeeded x1")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for success digest flush")
	}
}

func TestNotificationBatcher_ReplacesWaitingWithSuccessBeforeFlush(t *testing.T) {
	t.Parallel()

	var (
		mu      sync.Mutex
		flushed []NotificationBatch
	)
	batcher := NewNotificationBatcher(15*time.Millisecond, 25*time.Millisecond, func(_ string, batch NotificationBatch) {
		mu.Lock()
		defer mu.Unlock()
		flushed = append(flushed, batch)
	})
	defer batcher.Stop()

	require.True(t, batcher.Enqueue("dest-1", &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-1", AttemptID: "a1", Status: core.Waiting}))
	time.Sleep(5 * time.Millisecond)
	require.True(t, batcher.Enqueue("dest-1", &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-1", AttemptID: "a1", Status: core.Succeeded}))

	time.Sleep(20 * time.Millisecond)
	mu.Lock()
	currentFlushes := len(flushed)
	mu.Unlock()
	assert.Zero(t, currentFlushes, "waiting batch should have been replaced before urgent flush")

	require.Eventually(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(flushed) == 1
	}, time.Second, 10*time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	require.Len(t, flushed, 1)
	assert.Equal(t, NotificationClassSuccessDigest, flushed[0].Class)
	require.Len(t, flushed[0].Events, 1)
	assert.Equal(t, core.Succeeded, flushed[0].Events[0].Status.Status)
}

func TestNotificationBatcher_DuplicateStatusDoesNotDuplicateBatch(t *testing.T) {
	t.Parallel()

	type flushedBatch struct {
		destination string
		batch       NotificationBatch
	}

	flushCh := make(chan flushedBatch, 1)
	batcher := NewNotificationBatcher(20*time.Millisecond, 40*time.Millisecond, func(destination string, batch NotificationBatch) {
		flushCh <- flushedBatch{destination: destination, batch: batch}
	})
	defer batcher.Stop()

	status := &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-1", AttemptID: "a1", Status: core.Failed, Error: "boom"}
	require.True(t, batcher.Enqueue("dest-1", status))
	time.Sleep(10 * time.Millisecond)
	require.True(t, batcher.Enqueue("dest-1", status))

	select {
	case flushed := <-flushCh:
		assert.Equal(t, NotificationClassUrgent, flushed.batch.Class)
		require.Len(t, flushed.batch.Events, 1)
		assert.Equal(t, core.Failed, flushed.batch.Events[0].Status.Status)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for urgent flush")
	}
}

func TestNotificationBatcher_DrainAndStopReturnsPendingBatchesOrderedAndStopsFlushes(t *testing.T) {
	t.Parallel()

	var (
		mu         sync.Mutex
		flushCount int
	)
	batcher := NewNotificationBatcher(80*time.Millisecond, 120*time.Millisecond, func(_ string, _ NotificationBatch) {
		mu.Lock()
		defer mu.Unlock()
		flushCount++
	})

	require.True(t, batcher.Enqueue("success-dest", &exec.DAGRunStatus{
		Name:      "briefing",
		DAGRunID:  "run-1",
		AttemptID: "a1",
		Status:    core.Succeeded,
	}))
	time.Sleep(5 * time.Millisecond)
	require.True(t, batcher.Enqueue("urgent-old", &exec.DAGRunStatus{
		Name:      "sync",
		DAGRunID:  "run-2",
		AttemptID: "a2",
		Status:    core.Failed,
	}))
	time.Sleep(5 * time.Millisecond)
	require.True(t, batcher.Enqueue("urgent-new", &exec.DAGRunStatus{
		Name:      "sync",
		DAGRunID:  "run-3",
		AttemptID: "a3",
		Status:    core.Waiting,
	}))

	drained := batcher.DrainAndStop()
	require.Len(t, drained, 3)
	assert.Equal(t, "urgent-old", drained[0].Destination)
	assert.Equal(t, NotificationClassUrgent, drained[0].Batch.Class)
	assert.Equal(t, "urgent-new", drained[1].Destination)
	assert.Equal(t, NotificationClassUrgent, drained[1].Batch.Class)
	assert.Equal(t, "success-dest", drained[2].Destination)
	assert.Equal(t, NotificationClassSuccessDigest, drained[2].Batch.Class)

	time.Sleep(150 * time.Millisecond)
	mu.Lock()
	assert.Zero(t, flushCount)
	mu.Unlock()
	assert.False(t, batcher.Enqueue("ignored", &exec.DAGRunStatus{
		Name:      "ignored",
		DAGRunID:  "run-4",
		AttemptID: "a4",
		Status:    core.Succeeded,
	}))
}

func TestGenerateNotificationMessage_UrgentSingleUsesLLMAndFallsBack(t *testing.T) {
	t.Parallel()

	batch := NotificationBatch{
		Class: NotificationClassUrgent,
		Events: []NotificationEvent{
			{Status: &exec.DAGRunStatus{Name: "briefing", DAGRunID: "run-1", AttemptID: "a1", Status: core.Failed, Error: "boom"}},
		},
		WindowStart: time.Now().Add(-10 * time.Second),
		WindowEnd:   time.Now(),
	}

	service := &fakeAgentService{
		generatedMessage: agent.Message{Type: agent.MessageTypeAssistant, Content: "ai notification"},
	}
	msg, err := GenerateNotificationMessage(context.Background(), service, "sess-1", agent.UserIdentity{UserID: "u1"}, batch)
	require.NoError(t, err)
	assert.Equal(t, "ai notification", msg.Content)

	service.generateErr = errors.New("llm unavailable")
	msg, err = GenerateNotificationMessage(context.Background(), service, "sess-1", agent.UserIdentity{UserID: "u1"}, batch)
	require.Error(t, err)
	assert.Contains(t, msg.Content, "DAG `briefing` failed")
	assert.NotContains(t, msg.Content, "llm unavailable")
}

func TestFormatNotificationBatch_CapsVisibleGroups(t *testing.T) {
	t.Parallel()

	events := make([]NotificationEvent, 0, maxNotificationGroups+2)
	base := time.Now()
	for i := range maxNotificationGroups + 2 {
		events = append(events, NotificationEvent{
			Status: &exec.DAGRunStatus{
				Name:      "dag-" + string(rune('a'+i)),
				DAGRunID:  "run-" + string(rune('a'+i)),
				AttemptID: "a1",
				Status:    core.Succeeded,
			},
			ObservedAt: base.Add(-time.Duration(i) * time.Second),
		})
	}

	text := FormatNotificationBatch(NotificationBatch{
		Class:       NotificationClassSuccessDigest,
		Events:      events,
		WindowStart: base.Add(-2 * time.Minute),
		WindowEnd:   base,
	})

	assert.Contains(t, text, "DAG completion digest")
	assert.Contains(t, text, "and 2 more DAG groups")
}
