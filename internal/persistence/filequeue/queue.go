package filequeue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/dagu-org/dagu/internal/common/dirlock"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Errors for the queue
var (
	ErrQueueEmpty        = errors.New("queue is empty")
	ErrQueueItemNotFound = errors.New("queue item not found")
)

// priorities is a list of queue priorities
var priorities = []execution.QueuePriority{
	execution.QueuePriorityHigh, execution.QueuePriorityLow,
}

// DualQueue represents a queue for storing dag-runs with two priorities:
// high and low. It uses two queue files to store the items.
type DualQueue struct {
	dirlock.DirLock // Embed DirLock to ensure thread-safe access to the queue files

	// baseDir is the base directory for the queue files
	baseDir string
	// name is the name of the DAGs that this queue is for
	name string
	// queueFiles is a map of queue files, where the key is the priority
	files map[execution.QueuePriority]*QueueFile
	// mu is the mutex for synchronizing access to the queue
	mu sync.Mutex
}

// NewDualQueue creates a new queue with the specified base directory and name
// It initializes the queue files for high and low priority
func NewDualQueue(baseDir, name string) *DualQueue {
	dirLock := dirlock.New(baseDir, &dirlock.LockOptions{
		StaleThreshold: 30, // seconds
		RetryInterval:  50, // milliseconds
	})
	return &DualQueue{
		DirLock: dirLock,
		baseDir: baseDir,
		name:    name,
		files: map[execution.QueuePriority]*QueueFile{
			execution.QueuePriorityHigh: NewQueueFile(baseDir, "high_"),
			execution.QueuePriorityLow:  NewQueueFile(baseDir, "low_"),
		},
	}
}

// FindByDAGRunID retrieves a dag-run from the queue by its dag-run ID
// without removing it. It returns the first found item in the queue files.
// If the item is not found in any of the queue files, it returns ErrQueueItemNotFound.
func (q *DualQueue) FindByDAGRunID(ctx context.Context, dagRunID string) (execution.QueuedItemData, error) {
	for _, priority := range priorities {
		qf := q.files[priority]
		item, err := qf.FindByDAGRunID(ctx, dagRunID)
		if errors.Is(err, ErrQueueFileItemNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to find dag-run %s: %w", dagRunID, err)
		}
		return item, nil
	}
	return nil, ErrQueueItemNotFound
}

// DequeueByDAGRunID retrieves a dag-run from the queue by its dag-run ID
func (q *DualQueue) DequeueByDAGRunID(ctx context.Context, dagRun execution.DAGRunRef) ([]execution.QueuedItemData, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var items []execution.QueuedItemData
	for _, priority := range priorities {
		qf := q.files[priority]
		popped, err := qf.PopByDAGRunID(ctx, dagRun)
		if errors.Is(err, ErrQueueFileEmpty) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("failed to pop dag-run %s: %w", dagRun.ID, err)
		}
		for _, item := range popped {
			items = append(items, item)
		}
	}

	// Remove directory if it's empty
	_ = os.Remove(q.baseDir)

	return items, nil
}

// List returns all items in the queue
func (q *DualQueue) List(ctx context.Context) ([]execution.QueuedItemData, error) {
	var items []execution.QueuedItemData
	for _, priority := range priorities {
		qf := q.files[priority]
		qItems, err := qf.List(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list items in queue with priority %d: %w", priority, err)
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

// Enqueue adds a dag-run to the queue with the specified priority
func (q *DualQueue) Enqueue(ctx context.Context, priority execution.QueuePriority, dagRun execution.DAGRunRef) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if _, ok := q.files[priority]; !ok {
		return fmt.Errorf("invalid queue priority: %d", priority)
	}
	qf := q.files[priority]
	if err := qf.Push(ctx, dagRun); err != nil {
		return err
	}
	logger.Debug(ctx, "Enqueue", "dagRunId", dagRun.ID, "priority", priority)
	return nil
}

// Dequeue retrieves a dag-run from the queue and removes it.
// It checks the high-priority queue first, then the low-priority queue
func (q *DualQueue) Dequeue(ctx context.Context) (execution.QueuedItemData, error) {
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
