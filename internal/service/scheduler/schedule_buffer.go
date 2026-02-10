package scheduler

import (
	"time"

	"github.com/dagu-org/dagu/internal/core"
)

// QueueItem represents a scheduled or catch-up run to be dispatched.
type QueueItem struct {
	DAG           *core.DAG
	ScheduledTime time.Time
	TriggerType   core.TriggerType
	ScheduleType  ScheduleType
}

// ScheduleBuffer is a per-DAG in-memory FIFO queue for catch-up runs.
// It holds no concurrency primitives — all access happens from the cronLoop goroutine.
type ScheduleBuffer struct {
	items         []QueueItem
	overlapPolicy core.OverlapPolicy
	dagName       string
}

// NewScheduleBuffer creates a per-DAG queue with the given overlap policy.
func NewScheduleBuffer(dagName string, policy core.OverlapPolicy) *ScheduleBuffer {
	return &ScheduleBuffer{
		overlapPolicy: policy,
		dagName:       dagName,
	}
}

// Send appends an item to the back of the queue.
func (q *ScheduleBuffer) Send(item QueueItem) {
	q.items = append(q.items, item)
}

// Peek returns the front item without removing it.
// Returns false if the queue is empty.
func (q *ScheduleBuffer) Peek() (QueueItem, bool) {
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	return q.items[0], true
}

// Pop removes and returns the front item.
// Returns false if the queue is empty.
func (q *ScheduleBuffer) Pop() (QueueItem, bool) {
	if len(q.items) == 0 {
		return QueueItem{}, false
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

// Len returns the number of items in the queue.
func (q *ScheduleBuffer) Len() int {
	return len(q.items)
}
