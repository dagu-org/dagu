package exec

import (
	"context"
	"errors"
)

// Errors for the queue
var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrQueueItemNotFound = errors.New("queue item not found")
)

// QueueStore provides an interface for interacting with the underlying database
// for storing and retrieving queued dag-run items.
type QueueStore interface {
	// Enqueue adds an item to the queue
	Enqueue(ctx context.Context, name string, priority QueuePriority, dagRun DAGRunRef) error
	// DequeueByName retrieves an item from the queue and removes it
	DequeueByName(ctx context.Context, name string) (QueuedItemData, error)
	// DequeueByDAGRunID retrieves items from the queue by dag-run reference and removes them
	DequeueByDAGRunID(ctx context.Context, name string, dagRun DAGRunRef) ([]QueuedItemData, error)
	// Len returns the number of items in the queue
	Len(ctx context.Context, name string) (int, error)
	// List returns all items in the queue with the given name
	List(ctx context.Context, name string) ([]QueuedItemData, error)
	// ListPaginated returns paginated items for a specific queue
	ListPaginated(ctx context.Context, name string, pg Paginator) (PaginatedResult[QueuedItemData], error)
	// All returns all items in the queue
	All(ctx context.Context) ([]QueuedItemData, error)
	// ListByDAGName returns all items that has a specific DAG name
	ListByDAGName(ctx context.Context, name, dagName string) ([]QueuedItemData, error)
	// QueueList lists all queue names that have at least one item in the queue
	QueueList(ctx context.Context) ([]string, error)
	// Watcher returns a QueueWatcher for the queue data
	QueueWatcher(ctx context.Context) QueueWatcher
}

// QueueWatcher watches the queue state
type QueueWatcher interface {
	// Start start swatching queue data and signal when a queue state changed
	Start(ctx context.Context) (<-chan struct{}, error)
	// Stop stops watching queue data
	Stop(ctx context.Context)
}

// QueuePriority represents the priority of a queued item
type QueuePriority int

const (
	QueuePriorityHigh QueuePriority = iota
	QueuePriorityLow
)

// QueuedItem is a wrapper for QueuedItemData
type QueuedItem struct {
	QueuedItemData
}

// NewQueuedItem creates a new QueuedItem
func NewQueuedItem(data QueuedItemData) *QueuedItem {
	return &QueuedItem{
		QueuedItemData: data,
	}
}

// QueuedItemData represents a dag-run reference that is queued for execution.
type QueuedItemData interface {
	// ID returns the ID of the queued item
	ID() string
	// Data returns the data of the queued item
	Data() (*DAGRunRef, error)
}
