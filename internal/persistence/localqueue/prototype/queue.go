package prototype

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Errors for the queue
var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrQueueItemNotFound = errors.New("queue item not found")
)

// priorities is a list of queue priorities
var priorities = []models.QueuePriority{
	models.QueuePriorityHigh, models.QueuePriorityLow,
}

// DualQueue represents a queue for storing workflows with two priorities:
// high and low. It uses two queue files to store the workflows.
type DualQueue struct {
	// baseDir is the base directory for the queue files
	baseDir string
	// name is the name of the workflow
	name string
	// queueFiles is a map of queue files, where the key is the priority
	files map[models.QueuePriority]*QueueFile
	// mu is the mutex for synchronizing access to the queue
	mu sync.Mutex
}

// NewDualQueue creates a new queue with the specified base directory and name
// It initializes the queue files for high and low priority
func NewDualQueue(baseDir, name string) *DualQueue {
	return &DualQueue{
		baseDir: baseDir,
		name:    name,
		files: map[models.QueuePriority]*QueueFile{
			models.QueuePriorityHigh: NewQueueFile(baseDir, "high_"),
			models.QueuePriorityLow:  NewQueueFile(baseDir, "low_"),
		},
	}
}

// FindByWorkflowID retrieves a workflow from the queue by its ID
// without removing it. It returns the first found item in the queue files.
// If the workflow is not found, it returns ErrQueueItemNotFound.
func (q *DualQueue) FindByWorkflowID(ctx context.Context, workflowID string) (models.QueuedItemData, error) {
	for _, priority := range priorities {
		qf := q.files[priority]
		item, err := qf.FindByWorkflowID(ctx, workflowID)
		if errors.Is(err, ErrQueueFileItemNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to find workflow %s: %w", workflowID, err)
		}
		return item, nil
	}
	return nil, ErrQueueItemNotFound
}

// DequeueByWorkflowID retrieves a workflow from the queue by its ID and removes it
func (q *DualQueue) DequeueByWorkflowID(ctx context.Context, workflowID string) ([]models.QueuedItemData, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var items []models.QueuedItemData
	for _, priority := range priorities {
		qf := q.files[priority]
		popped, err := qf.PopByWorkflowID(ctx, workflowID)
		if errors.Is(err, ErrQueueFileEmpty) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to pop workflow %s: %w", workflowID, err)
		}
		for _, item := range popped {
			items = append(items, item)
		}
	}
	return items, nil
}

// List returns all items in the queue
func (q *DualQueue) List(ctx context.Context) ([]models.QueuedItemData, error) {
	var items []models.QueuedItemData
	for _, priority := range priorities {
		qf := q.files[priority]
		qItems, err := qf.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list workflows: %w", err)
		}
		for _, item := range qItems {
			items = append(items, item)
		}
	}
	return items, nil
}

// Len returns the total number of items in the queue
func (q *DualQueue) Len(ctx context.Context) (int, error) {
	var total int
	for _, priority := range priorities {
		qf := q.files[priority]
		l, err := qf.Len(ctx)
		if err != nil {
			return 0, err
		}
		total += l
	}
	return total, nil
}

// Enqueue adds a workflow to the queue with the specified priority
func (q *DualQueue) Enqueue(ctx context.Context, priority models.QueuePriority, workflow digraph.DAGRunRef) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.files[priority]; !ok {
		return fmt.Errorf("invalid queue priority: %d", priority)
	}
	qf := q.files[priority]
	if err := qf.Push(ctx, workflow); err != nil {
		return err
	}
	logger.Debug(ctx, "Enqueue", "dagRunId", workflow.ID, "priority", priority)
	return nil
}

// Dequeue retrieves a workflow from the queue and removes it
// It checks the high-priority queue first, then the low-priority queue
func (q *DualQueue) Dequeue(ctx context.Context) (models.QueuedItemData, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, priority := range priorities {
		qf := q.files[priority]
		item, err := qf.Pop(ctx)
		if errors.Is(err, ErrQueueFileEmpty) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if item != nil {
			logger.Debug(ctx, "Dequeue", "dagRunId", item.ID(), "priority", priority)
			return item, nil
		}
	}

	// Delete the directory if it's empty
	// It fails silently if the directory
	_ = os.Remove(q.baseDir)

	return nil, ErrQueueEmpty
}
