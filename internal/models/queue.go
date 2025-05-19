package models

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/digraph"
)

// Errors for the queue
var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrQueueItemNotFound = errors.New("queue item not found")
)

// QueueStore provides an interface for interacting with the underlying database
// for storing and retrieving queued workflows.
type QueueStore interface {
	// Enqueue adds an item to the queue
	Enqueue(ctx context.Context, name string, priority QueuePriority, workflow digraph.WorkflowRef) error
	// DequeueByName retrieves an item from the queue and removes it
	DequeueByName(ctx context.Context, name string) (QueuedItem, error)
	// Len returns the number of items in the queue
	DequeueByWorkflowID(ctx context.Context, workflowID string) ([]QueuedItem, error)
	// List returns all items in the queue
	Len(ctx context.Context, name string) (int, error)
	// DequeueByWorkflowID retrieves a workflow from the queue by its ID and removes it
	List(ctx context.Context, name string) ([]QueuedItem, error)
	// All returns all items in the queue
	All(ctx context.Context) ([]QueuedItem, error)
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
