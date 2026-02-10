// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestDAGQueue_BasicDispatch(t *testing.T) {
	t.Parallel()

	q := NewDAGQueue("test-dag", core.OverlapPolicyAll, 10)

	var dispatched []time.Time
	var mu sync.Mutex

	dispatch := func(_ context.Context, item QueueItem) error {
		mu.Lock()
		dispatched = append(dispatched, item.ScheduledTime)
		mu.Unlock()
		return nil
	}

	isRunning := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now()
	q.Send(QueueItem{ScheduledTime: now, TriggerType: core.TriggerTypeCatchUp})
	q.Send(QueueItem{ScheduledTime: now.Add(time.Hour), TriggerType: core.TriggerTypeCatchUp})
	q.Close()

	q.Start(ctx, dispatch, isRunning)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, dispatched, 2)
	assert.Equal(t, now, dispatched[0])
	assert.Equal(t, now.Add(time.Hour), dispatched[1])
}

func TestDAGQueue_SkipPolicy(t *testing.T) {
	t.Parallel()

	q := NewDAGQueue("test-dag", core.OverlapPolicySkip, 10)

	var dispatched []time.Time
	var mu sync.Mutex

	dispatch := func(_ context.Context, item QueueItem) error {
		mu.Lock()
		dispatched = append(dispatched, item.ScheduledTime)
		mu.Unlock()
		return nil
	}

	// First call: running, second call: not running
	callCount := 0
	var callMu sync.Mutex
	isRunning := func(_ context.Context, _ string) (bool, error) {
		callMu.Lock()
		defer callMu.Unlock()
		callCount++
		// First item: not running, second item: running (should be skipped)
		return callCount == 2, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now()
	q.Send(QueueItem{ScheduledTime: now, TriggerType: core.TriggerTypeCatchUp})
	q.Send(QueueItem{ScheduledTime: now.Add(time.Hour), TriggerType: core.TriggerTypeCatchUp})
	q.Close()

	q.Start(ctx, dispatch, isRunning)

	mu.Lock()
	defer mu.Unlock()
	// First should be dispatched, second skipped because isRunning returned true
	assert.Len(t, dispatched, 1)
	assert.Equal(t, now, dispatched[0])
}

func TestDAGQueue_OrderPreservation(t *testing.T) {
	t.Parallel()

	q := NewDAGQueue("test-dag", core.OverlapPolicyAll, 20)

	var dispatched []time.Time
	var mu sync.Mutex

	dispatch := func(_ context.Context, item QueueItem) error {
		mu.Lock()
		dispatched = append(dispatched, item.ScheduledTime)
		mu.Unlock()
		return nil
	}

	isRunning := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	base := time.Date(2026, 2, 7, 9, 0, 0, 0, time.UTC)
	for i := range 10 {
		q.Send(QueueItem{
			ScheduledTime: base.Add(time.Duration(i) * time.Hour),
			TriggerType:   core.TriggerTypeCatchUp,
		})
	}
	q.Close()

	q.Start(ctx, dispatch, isRunning)

	mu.Lock()
	defer mu.Unlock()
	assert.Len(t, dispatched, 10)
	// Verify chronological order
	for i := 1; i < len(dispatched); i++ {
		assert.True(t, dispatched[i].After(dispatched[i-1]))
	}
}

func TestDAGQueue_ContextCancellation(t *testing.T) {
	t.Parallel()

	q := NewDAGQueue("test-dag", core.OverlapPolicyAll, 10)

	dispatch := func(_ context.Context, _ QueueItem) error {
		return nil
	}

	isRunning := func(_ context.Context, _ string) (bool, error) {
		return false, nil
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		q.Start(ctx, dispatch, isRunning)
		close(done)
	}()

	// Cancel should cause Start to return
	cancel()

	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
