package models

import (
	"context"

	"github.com/dagu-org/dagu/internal/digraph"
)

// QueueStorage provides an interface for interacting with the underlying database
// for storing and retrieving queued workflows.
type QueueStorage interface {
	// Enqueue adds an item to the queue
	Enqueue(ctx context.Context, priority QueuePriority, name string, workflow digraph.WorkflowRef) error
	// Dequeue retrieves an item from the queue and removes it
	Dequeue(ctx context.Context, name, id string) (QueuedItem, error)
}

// QueuePriority represents the priority of a queued item
type QueuePriority int

const (
	QueuePriorityHigh QueuePriority = iota
	QueuePriorityLow
)

// QueuedItem represents a workflow that is in the queue for execution
type QueuedItem interface {
	// ID returns the ID of the queued item
	ID() string
	// Data returns the data of the queued item
	Data() (*digraph.WorkflowRef, error)
}
