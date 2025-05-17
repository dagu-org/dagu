package prototype

import (
	"context"
	"errors"
	"fmt"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Errors for the queue
var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrQueueItemNotFound = errors.New("queue item not found")
)

// Queue represents a queue for storing workflows with different priorities
type Queue struct {
	// baseDir is the base directory for the queue files
	baseDir string
	// name is the name of the workflow
	name string
	// queueFiles is a map of queue files, where the key is the priority
	files map[models.QueuePriority]*QueueFile
}

// NewQueue creates a new queue with the specified base directory and name
func NewQueue(baseDir, name string) *Queue {
	return &Queue{
		baseDir: baseDir,
		name:    name,
		files: map[models.QueuePriority]*QueueFile{
			models.QueuePriorityHigh: NewQueueFile(baseDir, name, "high"),
			models.QueuePriorityLow:  NewQueueFile(baseDir, name, "low"),
		},
	}
}

// FindByWorkflowID retrieves a workflow from the queue by its ID
// without removing it. It returns the first found item in the queue files.
// If the workflow is not found, it returns ErrQueueItemNotFound.
func (q *Queue) FindByWorkflowID(ctx context.Context, workflowID string) (models.QueuedItem, error) {
	for _, qf := range q.files {
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
func (q *Queue) DequeueByWorkflowID(ctx context.Context, workflowID string) ([]models.QueuedItem, error) {
	var items []models.QueuedItem
	for _, qf := range q.files {
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

// Len returns the total number of items in the queue
func (q *Queue) Len(ctx context.Context) (int, error) {
	var total int
	for _, qf := range q.files {
		l, err := qf.Len(ctx)
		if err != nil {
			return 0, err
		}
		total += l
	}
	return total, nil
}

// Enqueue adds a workflow to the queue with the specified priority
func (q *Queue) Enqueue(ctx context.Context, priority models.QueuePriority, workflow digraph.WorkflowRef) error {
	qf := q.files[priority]
	if err := qf.Push(ctx, workflow); err != nil {
		return err
	}
	logger.Debug(ctx, "Enqueue", "workflow", workflow.WorkflowID, "priority", priority)
	return nil
}

// Dequeue retrieves a workflow from the queue and removes it
func (q *Queue) Dequeue(ctx context.Context) (models.QueuedItem, error) {
	for priority, qf := range q.files {
		item, err := qf.Pop(ctx)
		if errors.Is(err, ErrQueueFileEmpty) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if item != nil {
			logger.Debug(ctx, "Dequeue", "workflow", item.ID(), "priority", priority)
			return item, nil
		}
	}
	return nil, ErrQueueEmpty
}
