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
// for storing and retrieving queued dag-run items.
type QueueStore interface {
	// Enqueue adds an item to the queue
	Enqueue(ctx context.Context, name string, priority QueuePriority, dagRun digraph.DAGRunRef) error
	// DequeueByName retrieves an item from the queue and removes it
	DequeueByName(ctx context.Context, name string) (QueuedItemData, error)
	// DequeueByDAGRunID retrieves items from the queue by dag-run ID and removes them
	DequeueByDAGRunID(ctx context.Context, name, dagRunID string) ([]QueuedItemData, error)
	// Len returns the number of items in the queue
	Len(ctx context.Context, name string) (int, error)
	// List returns all items in the queue with the given name
	List(ctx context.Context, name string) ([]QueuedItemData, error)
	// All returns all items in the queue
	All(ctx context.Context) ([]QueuedItemData, error)
	// Reader returns a QueueReader for reading from the queue
	Reader(ctx context.Context) QueueReader
}

// QueueReader provides an interface for reading from the queue
type QueueReader interface {
	// Start starts the queue reader
	Start(ctx context.Context, ch chan<- QueuedItem) error
	// Stop stops the queue reader
	Stop(ctx context.Context)
	// IsRunning returns true if the queue reader is running
	IsRunning() bool
}

// QueuePriority represents the priority of a queued item
type QueuePriority int

const (
	QueuePriorityHigh QueuePriority = iota
	QueuePriorityLow
)

// QueuedItem is a wrapper for QueuedItem with additional fields
type QueuedItem struct {
	QueuedItemData
	Result chan QueuedItemProcessingResult
}

type QueuedItemProcessingResult int

const (
	// QueuedItemProcessingResultRetry indicates that the queued item needs to be retried
	QueuedItemProcessingResultRetry QueuedItemProcessingResult = 0
	// QueuedItemProcessingResultSuccess indicates that the queued item was processed successfully
	QueuedItemProcessingResultSuccess QueuedItemProcessingResult = 1
	// QueuedItemProcessingResultInvalid indicates that the queued item was invalid so it needs to be removed
	QueuedItemProcessingResultInvalid QueuedItemProcessingResult = 2
)

// NewQueuedItem creates a new QueuedItem
func NewQueuedItem(data QueuedItemData) *QueuedItem {
	return &QueuedItem{
		QueuedItemData: data,
		Result:         make(chan QueuedItemProcessingResult, 1),
	}
}

// QueuedItemData represents a dag-run reference that is queued for execution.
type QueuedItemData interface {
	// ID returns the ID of the queued item
	ID() string
	// Data returns the data of the queued item
	Data() digraph.DAGRunRef
}
