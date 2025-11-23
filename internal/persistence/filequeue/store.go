package filequeue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
)

func New(baseDir string) execution.QueueStore {
	return &Store{
		baseDir: baseDir,
		queues:  make(map[string]*DualQueue),
	}
}

var _ execution.QueueStore = (*Store)(nil)

// Store implements models.QueueStore.
// It provides a dead-simple queue implementation using files.
// Since implementing a queue is not trivial, this implementation provides
// as a prototype for a more complex queue implementation.
type Store struct {
	baseDir string
	// queues is a map of queues, where the key is the queue name (DAG name)
	queues map[string]*DualQueue
	mu     sync.Mutex
}

// QueueWatcher implements execution.QueueStore.
func (s *Store) QueueWatcher(_ context.Context) execution.QueueWatcher {
	return newWatcher(s.baseDir)
}

// QueueList lists all queue names that have at least one item in the queue.
func (s *Store) QueueList(ctx context.Context) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var names []string
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		logger.Error(ctx, "Failed to read base directory",
			tag.Dir(s.baseDir),
			tag.Error(err))
		return nil, fmt.Errorf("queue: failed to read base directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			name := entry.Name()
			// Ensure the directory is not empty
			queueDir := filepath.Join(s.baseDir, name)
			subEntries, err := os.ReadDir(queueDir)
			if err != nil {
				logger.Error(ctx, "Failed to read queue directory",
					tag.Dir(queueDir),
					tag.Error(err))
				continue
			}
			if len(subEntries) == 0 {
				continue
			}
			names = append(names, name)
		}
	}

	return names, nil
}

// All implements models.QueueStore.
func (s *Store) All(ctx context.Context) ([]execution.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []execution.QueuedItemData

	patterns := []string{
		filepath.Join(s.baseDir, "*", "item_high_*.json"),
		filepath.Join(s.baseDir, "*", "item_low_*.json"),
	}

	for _, pattern := range patterns {
		// Grep high priority items in the directory
		files, err := filepath.Glob(pattern)
		if err != nil {
			logger.Error(ctx, "Failed to list queue files", tag.Error(err))
			return nil, fmt.Errorf("failed to list high priority dag-runs: %w", err)
		}

		// Sort the files by name which reflects the order of the items
		sort.Strings(files)

		for _, file := range files {
			data, err := parseQueueFileName(file, filepath.Base(file))
			if err != nil {
				logger.Error(ctx, "Failed to parse queue file name",
					tag.File(file),
					tag.Error(err))
				continue
			}
			items = append(items, NewJob(data))
		}
	}

	return items, nil
}

// DequeueByName implements models.QueueStore.
func (s *Store) DequeueByName(ctx context.Context, name string) (execution.QueuedItemData, error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	if err := s.queues[name].Lock(ctx); err != nil {
		logger.Error(ctx, "Failed to lock queue", tag.Error(err))
		return nil, fmt.Errorf("failed to lock queue %s: %w", name, err)
	}
	defer func() {
		if err := s.queues[name].Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock queue",
				tag.Queue(name),
				tag.Error(err))
		}
	}()

	q := s.queues[name]
	item, err := q.Dequeue(ctx)
	if errors.Is(err, ErrQueueEmpty) {
		return nil, execution.ErrQueueEmpty
	}

	if err != nil {
		logger.Error(ctx, "Failed to dequeue dag-run", tag.Error(err))
		return nil, fmt.Errorf("failed to dequeue dag-run %s: %w", name, err)
	}

	return item, nil
}

// Len implements models.QueueStore.
func (s *Store) Len(ctx context.Context, name string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	return q.Len(ctx)
}

// List implements models.QueueStore.
func (s *Store) List(ctx context.Context, name string) ([]execution.QueuedItemData, error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	items, err := q.List(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list dag-runs", tag.Error(err))
		return nil, fmt.Errorf("failed to list dag-runs %s: %w", name, err)
	}

	return items, nil
}

func (s *Store) ListByDAGName(ctx context.Context, name, dagName string) ([]execution.QueuedItemData, error) {
	items, err := s.List(ctx, name)
	if err != nil {
		return nil, err
	}
	var ret []execution.QueuedItemData
	for _, item := range items {
		if item.Data().Name == dagName {
			ret = append(ret, item)
		}
	}
	return ret, nil
}

// DequeueByDAGRunID implements models.QueueStore.
func (s *Store) DequeueByDAGRunID(ctx context.Context, name string, dagRun execution.DAGRunRef) ([]execution.QueuedItemData, error) {
	ctx = logger.WithValues(ctx,
		tag.Queue(name),
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
	)
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []execution.QueuedItemData
	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	if err := s.queues[name].Lock(ctx); err != nil {
		logger.Error(ctx, "Failed to lock queue", tag.Error(err))
		return nil, fmt.Errorf("failed to lock queue %s: %w", name, err)
	}
	defer func() {
		if err := s.queues[name].Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock queue",
				tag.Queue(name),
				tag.Error(err))
		}
	}()

	q := s.queues[name]
	item, err := q.DequeueByDAGRunID(ctx, dagRun)
	if err != nil {
		logger.Error(ctx, "Failed to dequeue dag-run by ID", tag.Error(err))
		return nil, fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
	}
	items = append(items, item...)

	if len(items) == 0 {
		return nil, execution.ErrQueueItemNotFound
	}

	return items, nil
}

// Enqueue implements models.QueueStore.
func (s *Store) Enqueue(ctx context.Context, name string, p execution.QueuePriority, dagRun execution.DAGRunRef) error {
	ctx = logger.WithValues(ctx,
		tag.Queue(name),
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
	)
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	if err := q.Enqueue(ctx, p, dagRun); err != nil {
		logger.Error(ctx, "Failed to enqueue dag-run",
			tag.Error(err),
			tag.Priority(int(p)),
		)
		return fmt.Errorf("failed to enqueue dag-run %s: %w", name, err)
	}

	logger.Info(ctx, "Enqueued dag-run",
		tag.Priority(int(p)),
	)
	return nil
}

// createDualQueue creates a new DualQueue for the given name.
func (s *Store) createDualQueue(name string) *DualQueue {
	queueBaseDir := filepath.Join(s.baseDir, name)
	return NewDualQueue(queueBaseDir, name)
}

// BaseDir returns the base directory of the queue store
func (s *Store) BaseDir() string {
	return s.baseDir
}
