package filequeue

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueueStore = (*Store)(nil)

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

// All implements models.QueueStore.
func (s *Store) All(ctx context.Context) ([]models.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []models.QueuedItemData

	patterns := []string{
		filepath.Join(s.baseDir, "*", "item_high_*.json"),
		filepath.Join(s.baseDir, "*", "item_low_*.json"),
	}

	for _, pattern := range patterns {
		// Grep high priority items in the directory
		files, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("failed to list high priority dag-runs: %w", err)
		}

		// Sort the files by name which reflects the order of the items
		sort.Strings(files)

		for _, file := range files {
			data, err := parseQueueFileName(file, filepath.Base(file))
			if err != nil {
				logger.Error(ctx, "Failed to parse queue file name", "file", file, "err", err)
				continue
			}
			items = append(items, NewJob(data))
		}
	}

	return items, nil
}

// DequeueByName implements models.QueueStore.
func (s *Store) DequeueByName(ctx context.Context, name string) (models.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	if err := s.queues[name].Lock(ctx); err != nil {
		return nil, fmt.Errorf("failed to lock queue %s: %w", name, err)
	}
	defer func() {
		if err := s.queues[name].Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock queue", "queue", name, "err", err)
		}
	}()

	q := s.queues[name]
	item, err := q.Dequeue(ctx)
	if errors.Is(err, ErrQueueEmpty) {
		return nil, models.ErrQueueEmpty
	}

	if err != nil {
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
func (s *Store) List(ctx context.Context, name string) ([]models.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	items, err := q.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list dag-runs %s: %w", name, err)
	}

	return items, nil
}

func (s *Store) ListByDAGName(ctx context.Context, name, dagName string) ([]models.QueuedItemData, error) {
	items, err := s.List(ctx, name)
	if err != nil {
		return nil, err
	}
	var ret []models.QueuedItemData
	for _, item := range items {
		if item.Data().Name == dagName {
			ret = append(ret, item)
		}
	}
	return ret, nil
}

// DequeueByDAGRunID implements models.QueueStore.
func (s *Store) DequeueByDAGRunID(ctx context.Context, name, dagRunID string) ([]models.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []models.QueuedItemData
	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	if err := s.queues[name].Lock(ctx); err != nil {
		return nil, fmt.Errorf("failed to lock queue %s: %w", name, err)
	}
	defer func() {
		if err := s.queues[name].Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock queue", "queue", name, "err", err)
		}
	}()

	q := s.queues[name]
	item, err := q.DequeueByDAGRunID(ctx, dagRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to dequeue dag-run %s: %w", dagRunID, err)
	}
	items = append(items, item...)

	if len(items) == 0 {
		return nil, models.ErrQueueItemNotFound
	}

	return items, nil
}

// Enqueue implements models.QueueStore.
func (s *Store) Enqueue(ctx context.Context, name string, p models.QueuePriority, dagRun core.DAGRunRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	if err := q.Enqueue(ctx, p, dagRun); err != nil {
		return fmt.Errorf("failed to enqueue dag-run %s: %w", name, err)
	}

	return nil
}

// Reader implements models.QueueStore.
func (s *Store) Reader(_ context.Context) models.QueueReader {
	return newQueueReader(s)
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

func New(baseDir string) models.QueueStore {
	return &Store{
		baseDir: baseDir,
		queues:  make(map[string]*DualQueue),
	}
}
