package scheduler

import (
	"context"
	"sync"
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
	mu            sync.Mutex
	cond          *sync.Cond
	items         []QueueItem
	closed        bool
	overlapPolicy core.OverlapPolicy
	dagName       string
}

// NewDAGQueue creates a per-DAG queue with the given overlap policy.
func NewDAGQueue(dagName string, policy core.OverlapPolicy) *DAGQueue {
	q := &DAGQueue{
		overlapPolicy: policy,
		dagName:       dagName,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Send enqueues an item (catch-up or live scheduled run).
func (q *DAGQueue) Send(item QueueItem) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.closed {
		return
	}
	q.items = append(q.items, item)
	q.cond.Signal()
}

// Start runs the consumer loop. Pops items in FIFO order, respects
// overlapPolicy, dispatches via the provided function. Exits when ctx is
// cancelled or the queue is closed and drained.
func (q *DAGQueue) Start(ctx context.Context, dispatch DispatchFunc, isRunning IsRunningFunc) {
	// Wake the consumer when the context is cancelled.
	go func() {
		<-ctx.Done()
		q.cond.Broadcast()
	}()

	for {
		item, ok := q.dequeue(ctx)
		if !ok {
			return
		}
		q.processItem(ctx, item, dispatch, isRunning)
	}
}

// Close signals that no more items will be sent.
func (q *DAGQueue) Close() {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.closed = true
	q.cond.Broadcast()
}

// dequeue blocks until an item is available, the queue is closed and empty,
// or the context is cancelled.
func (q *DAGQueue) dequeue(ctx context.Context) (QueueItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for len(q.items) == 0 && !q.closed && ctx.Err() == nil {
		q.cond.Wait()
	}

	if ctx.Err() != nil || (len(q.items) == 0 && q.closed) {
		return QueueItem{}, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
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
		// Fallback: dispatch immediately
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
