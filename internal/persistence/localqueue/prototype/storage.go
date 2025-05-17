package prototype

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueueStorage = (*Storage)(nil)

// Storage implements models.QueueStorage.
// It provides a dead-simple queue implementation using files.
// Since implementing a queue is not trivial, this implementation provides
// as a prototype for a more complex queue implementation.
type Storage struct {
	baseDir string
	// queues is a map of queues, where the key is the queue name (workflow name)
	queues map[string]*DualQueue
	mu     sync.Mutex
}

// Dequeue implements models.QueueStorage.
func (s *Storage) Dequeue(ctx context.Context, name string) (models.QueuedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	item, err := q.Dequeue(ctx)
	if errors.Is(err, ErrQueueEmpty) {
		return nil, models.ErrQueueEmpty
	}

	if err != nil {
		return nil, fmt.Errorf("failed to dequeue workflow %s: %w", name, err)
	}

	return item, nil
}

// Len implements models.QueueStorage.
func (s *Storage) Len(ctx context.Context, name string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	return q.Len(ctx)
}

// List implements models.QueueStorage.
func (s *Storage) List(ctx context.Context, name string) ([]models.QueuedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	items, err := q.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list workflows %s: %w", name, err)
	}

	return items, nil
}

// DequeueByWorkflowID implements models.QueueStorage.
func (s *Storage) DequeueByWorkflowID(ctx context.Context, workflowID string) ([]models.QueuedItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []models.QueuedItem
	for _, q := range s.queues {
		item, err := q.DequeueByWorkflowID(ctx, workflowID)
		if err != nil {
			return nil, fmt.Errorf("failed to dequeue workflow %s: %w", workflowID, err)
		}
		items = append(items, item...)
	}

	if len(items) == 0 {
		return nil, models.ErrQueueItemNotFound
	}

	return items, nil
}

// Enqueue implements models.QueueStorage.
func (s *Storage) Enqueue(ctx context.Context, name string, p models.QueuePriority, workflow digraph.WorkflowRef) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	q := s.queues[name]
	if err := q.Enqueue(ctx, p, workflow); err != nil {
		return fmt.Errorf("failed to enqueue workflow %s: %w", name, err)
	}

	return nil
}

func (s *Storage) createDualQueue(name string) *DualQueue {
	queueBaseDir := filepath.Join(s.baseDir, name)
	return NewDualQueue(queueBaseDir, name)
}

func New(baseDir string) models.QueueStorage {
	return &Storage{
		baseDir: baseDir,
		queues:  make(map[string]*DualQueue),
	}
}
