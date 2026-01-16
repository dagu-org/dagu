package filequeue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core/exec"
)

func New(baseDir string) exec.QueueStore {
	return &Store{
		baseDir: baseDir,
		queues:  make(map[string]*DualQueue),
	}
}

var _ exec.QueueStore = (*Store)(nil)

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
func (s *Store) QueueWatcher(_ context.Context) exec.QueueWatcher {
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
func (s *Store) All(ctx context.Context) ([]exec.QueuedItemData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []exec.QueuedItemData

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
			items = append(items, NewQueuedFile(file))
		}
	}

	return items, nil
}

// DequeueByName implements models.QueueStore.
func (s *Store) DequeueByName(ctx context.Context, name string) (exec.QueuedItemData, error) {
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
		return nil, exec.ErrQueueEmpty
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
func (s *Store) List(ctx context.Context, name string) ([]exec.QueuedItemData, error) {
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

// ListPaginated returns paginated items for a specific queue.
// This implementation paginates at the file-path level BEFORE creating any
// QueuedFile objects, ensuring O(1) memory for the paginated items regardless
// of total queue size.
func (s *Store) ListPaginated(ctx context.Context, name string, pg exec.Paginator) (exec.PaginatedResult[exec.QueuedItemData], error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	limit := pg.Limit()
	offset := pg.Offset()

	// Build queue directory path
	queueDir := filepath.Join(s.baseDir, name)
	if _, err := os.Stat(queueDir); os.IsNotExist(err) {
		return exec.NewPaginatedResult([]exec.QueuedItemData{}, 0, pg), nil
	}

	// Collect file paths ONLY (no parsing, no object creation)
	// High priority first, then low - maintains proper queue ordering
	patterns := []string{
		filepath.Join(queueDir, "item_high_*.json"),
		filepath.Join(queueDir, "item_low_*.json"),
	}

	var allFiles []string
	for _, pattern := range patterns {
		files, err := filepath.Glob(pattern)
		if err != nil {
			logger.Error(ctx, "Failed to glob queue files", tag.Error(err))
			return exec.PaginatedResult[exec.QueuedItemData]{}, fmt.Errorf("failed to list queue files: %w", err)
		}
		// Lexicographic sort = chronological (timestamp encoded in filename)
		sort.Strings(files)
		allFiles = append(allFiles, files...)
	}

	total := len(allFiles)

	// Handle offset beyond total
	if offset >= total {
		return exec.NewPaginatedResult([]exec.QueuedItemData{}, total, pg), nil
	}

	// Apply pagination TO FILE PATHS (efficient - just string slicing)
	endIndex := offset + limit
	if endIndex > total {
		endIndex = total
	}
	paginatedFiles := allFiles[offset:endIndex]

	// Create QueuedFile objects ONLY for paginated portion
	// QueuedFile is lazy-loaded - JSON not read until Data() called
	items := make([]exec.QueuedItemData, 0, len(paginatedFiles))
	for _, file := range paginatedFiles {
		items = append(items, NewQueuedFile(file))
	}

	return exec.NewPaginatedResult(items, total, pg), nil
}

func (s *Store) ListByDAGName(ctx context.Context, name, dagName string) ([]exec.QueuedItemData, error) {
	items, err := s.List(ctx, name)
	if err != nil {
		return nil, err
	}
	var ret []exec.QueuedItemData
	for _, item := range items {
		data, err := item.Data()
		if err != nil {
			logger.Error(ctx, "Failed to get item data", tag.Error(err))
			continue
		}
		if data.Name == dagName {
			ret = append(ret, item)
		}
	}
	return ret, nil
}

// DequeueByDAGRunID implements models.QueueStore.
func (s *Store) DequeueByDAGRunID(ctx context.Context, name string, dagRun exec.DAGRunRef) ([]exec.QueuedItemData, error) {
	ctx = logger.WithValues(ctx,
		tag.Queue(name),
		tag.DAG(dagRun.Name),
		tag.RunID(dagRun.ID),
	)
	s.mu.Lock()
	defer s.mu.Unlock()

	var items []exec.QueuedItemData
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
		return nil, exec.ErrQueueItemNotFound
	}

	return items, nil
}

// Enqueue implements models.QueueStore.
func (s *Store) Enqueue(ctx context.Context, name string, p exec.QueuePriority, dagRun exec.DAGRunRef) error {
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

	logger.Info(ctx, "Enqueued dag-run", tag.Priority(int(p)))
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
