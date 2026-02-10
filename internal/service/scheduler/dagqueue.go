// Copyright 2024 The Dagu Authors
//
// Licensed under the GNU Affero General Public License, Version 3.0.

package scheduler

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
)

// DispatchFunc dispatches a queued item for execution.
// Returns an error if the dispatch fails.
type DispatchFunc func(ctx context.Context, item QueueItem) error

// IsRunningFunc checks whether the DAG has any active run (any trigger type).
type IsRunningFunc func(ctx context.Context, dagName string) (bool, error)

// QueueItem represents a scheduled or catch-up run to be dispatched.
type QueueItem struct {
	DAG           *core.DAG
	ScheduledTime time.Time
	TriggerType   core.TriggerType
	ScheduleType  ScheduleType
}

// DAGQueue is a per-DAG in-memory queue that unifies scheduled runs and
// catch-up runs into one dispatch mechanism. OverlapPolicy is enforced at the
// queue level.
type DAGQueue struct {
	ch            chan QueueItem
	overlapPolicy core.OverlapPolicy
	dagName       string
}

// NewDAGQueue creates a queue with a buffered channel.
func NewDAGQueue(dagName string, policy core.OverlapPolicy, bufSize int) *DAGQueue {
	if bufSize < 1 {
		bufSize = 1
	}
	return &DAGQueue{
		ch:            make(chan QueueItem, bufSize),
		overlapPolicy: policy,
		dagName:       dagName,
	}
}

// Send enqueues an item (catch-up or live scheduled run).
// Non-blocking: drops the item if the channel is full.
func (q *DAGQueue) Send(item QueueItem) {
	select {
	case q.ch <- item:
	default:
		// Channel full — item is dropped. This should not happen
		// because the buffer is sized to fit all catch-up items + live ticks.
	}
}

// Start runs the consumer goroutine. Reads from channel, respects overlapPolicy,
// dispatches via the provided function. Exits when ctx is cancelled or channel
// is closed.
func (q *DAGQueue) Start(ctx context.Context, dispatch DispatchFunc, isRunning IsRunningFunc) {
	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-q.ch:
			if !ok {
				return
			}
			q.processItem(ctx, item, dispatch, isRunning)
		}
	}
}

// Close closes the channel (signals no more items).
func (q *DAGQueue) Close() {
	close(q.ch)
}

func (q *DAGQueue) processItem(ctx context.Context, item QueueItem, dispatch DispatchFunc, isRunning IsRunningFunc) {
	switch q.overlapPolicy {
	case core.OverlapPolicySkip:
		running, err := isRunning(ctx, q.dagName)
		if err != nil {
			logger.Error(ctx, "Failed to check if DAG is running",
				tag.DAG(q.dagName),
				tag.Error(err),
			)
			return
		}
		if running {
			logger.Info(ctx, "Catch-up run skipped (overlap policy: skip)",
				tag.DAG(q.dagName),
			)
			return
		}
		if err := dispatch(ctx, item); err != nil {
			logger.Error(ctx, "Failed to dispatch queued item",
				tag.DAG(q.dagName),
				tag.Error(err),
			)
		}

	case core.OverlapPolicyAll:
		// Wait until the DAG is not running, then dispatch.
		if err := q.waitUntilNotRunning(ctx, isRunning); err != nil {
			return
		}
		if err := dispatch(ctx, item); err != nil {
			logger.Error(ctx, "Failed to dispatch queued item",
				tag.DAG(q.dagName),
				tag.Error(err),
			)
		}

	default:
		// Fallback: treat as skip
		if err := dispatch(ctx, item); err != nil {
			logger.Error(ctx, "Failed to dispatch queued item",
				tag.DAG(q.dagName),
				tag.Error(err),
			)
		}
	}
}

// waitUntilNotRunning polls with backoff until the DAG has no active runs.
func (q *DAGQueue) waitUntilNotRunning(ctx context.Context, isRunning IsRunningFunc) error {
	const (
		initialInterval = 2 * time.Second
		maxInterval     = 30 * time.Second
		backoffFactor   = 2
	)

	interval := initialInterval
	for {
		running, err := isRunning(ctx, q.dagName)
		if err != nil {
			logger.Error(ctx, "Failed to check if DAG is running while waiting",
				tag.DAG(q.dagName),
				tag.Error(err),
			)
			return err
		}
		if !running {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}

		interval *= backoffFactor
		if interval > maxInterval {
			interval = maxInterval
		}
	}
}
