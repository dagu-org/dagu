// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filequeue

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/dagucloud/dagu/internal/cmn/logger"
	"github.com/dagucloud/dagu/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/internal/core/exec"
)

func New(baseDir string) exec.QueueStore {
	return &Store{
		baseDir: baseDir,
		indices: make(map[string]*queueReadIndexCache),
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
	indices map[string]*queueReadIndexCache
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
			// Ensure the directory still contains queue item files.
			queueDir := filepath.Join(s.baseDir, name)
			subEntries, err := os.ReadDir(queueDir)
			if err != nil {
				logger.Error(ctx, "Failed to read queue directory",
					tag.Dir(queueDir),
					tag.Error(err))
				continue
			}
			if !hasQueueItemEntries(subEntries) {
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
	if item != nil {
		idx, idxErr := s.loadOrRebuildQueueIndexLocked(ctx, name)
		if idxErr != nil {
			logger.Warn(ctx, "Failed to refresh queue index after dequeue", tag.Error(idxErr))
			s.invalidateQueueIndexLocked(ctx, name)
			return item, nil
		}
		idx.removeItemID(item.ID())
		if saveErr := s.saveQueueIndexLocked(ctx, name, idx); saveErr != nil {
			logger.Warn(ctx, "Failed to persist queue index after dequeue", tag.Error(saveErr))
			s.invalidateQueueIndexLocked(ctx, name)
		}
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
	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		return 0, err
	}
	return idx.total(), nil
}

// List implements models.QueueStore.
func (s *Store) List(ctx context.Context, name string) ([]exec.QueuedItemData, error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}
	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		logger.Error(ctx, "Failed to load queue index", tag.Error(err))
		return nil, fmt.Errorf("failed to list dag-runs %s: %w", name, err)
	}
	queueDir := s.queueDir(name)
	items := make([]exec.QueuedItemData, 0, idx.total())
	for _, fileName := range idx.High {
		items = append(items, NewQueuedFile(filepath.Join(queueDir, fileName)))
	}
	for _, fileName := range idx.Low {
		items = append(items, NewQueuedFile(filepath.Join(queueDir, fileName)))
	}
	return items, nil
}

// ListCursor returns one forward-only page of queued items for a specific queue.
func (s *Store) ListCursor(ctx context.Context, name, cursor string, limit int) (exec.CursorResult[exec.QueuedItemData], error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 1
	}

	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		logger.Error(ctx, "Failed to load queue index", tag.Error(err))
		return exec.CursorResult[exec.QueuedItemData]{}, fmt.Errorf("failed to list queue files: %w", err)
	}

	decoded, err := decodeQueueReadCursor(name, cursor)
	if err != nil {
		return exec.CursorResult[exec.QueuedItemData]{}, err
	}
	start, err := idx.resolveStart(decoded)
	if err != nil {
		return exec.CursorResult[exec.QueuedItemData]{}, err
	}

	files := idx.slice(start, limit)
	items := make([]exec.QueuedItemData, 0, len(files))
	queueDir := s.queueDir(name)
	for _, fileName := range files {
		items = append(items, NewQueuedFile(filepath.Join(queueDir, fileName)))
	}

	hasMore := start+len(files) < idx.total()
	nextCursor := ""
	if hasMore && len(files) > 0 {
		nextCursor = encodeQueueReadCursor(name, idx, start+len(files), files[len(files)-1])
	}

	return exec.CursorResult[exec.QueuedItemData]{
		Items:      items,
		HasMore:    hasMore,
		NextCursor: nextCursor,
	}, nil
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
	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		logger.Warn(ctx, "Failed to refresh queue index after dequeue by dag-run ID", tag.Error(err))
		s.invalidateQueueIndexLocked(ctx, name)
		return items, nil
	}
	for _, queuedItem := range items {
		idx.removeItemID(queuedItem.ID())
	}
	if err := s.saveQueueIndexLocked(ctx, name, idx); err != nil {
		logger.Warn(ctx, "Failed to persist queue index after dequeue by dag-run ID", tag.Error(err))
		s.invalidateQueueIndexLocked(ctx, name)
	}

	return items, nil
}

// DeleteByItemIDs removes the exact queue items identified by their queue item IDs.
func (s *Store) DeleteByItemIDs(ctx context.Context, name string, itemIDs []string) (int, error) {
	ctx = logger.WithValues(ctx, tag.Queue(name))
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.queues[name]; !ok {
		s.queues[name] = s.createDualQueue(name)
	}

	if err := s.queues[name].Lock(ctx); err != nil {
		logger.Error(ctx, "Failed to lock queue", tag.Error(err))
		return 0, fmt.Errorf("failed to lock queue %s: %w", name, err)
	}
	defer func() {
		if err := s.queues[name].Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock queue",
				tag.Queue(name),
				tag.Error(err))
		}
	}()

	deleted, err := s.queues[name].DeleteByItemIDs(ctx, itemIDs)
	if err != nil {
		return deleted, err
	}
	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		logger.Warn(ctx, "Failed to refresh queue index after delete", tag.Error(err))
		s.invalidateQueueIndexLocked(ctx, name)
		return deleted, nil
	}
	for _, itemID := range itemIDs {
		idx.removeItemID(itemID)
	}
	if err := s.saveQueueIndexLocked(ctx, name, idx); err != nil {
		logger.Warn(ctx, "Failed to persist queue index after delete", tag.Error(err))
		s.invalidateQueueIndexLocked(ctx, name)
	}
	return deleted, nil
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

	idx, err := s.loadOrRebuildQueueIndexLocked(ctx, name)
	if err != nil {
		return err
	}

	q := s.queues[name]
	fileName, err := q.Enqueue(ctx, p, dagRun)
	if err != nil {
		logger.Error(ctx, "Failed to enqueue dag-run",
			tag.Error(err),
			tag.Priority(int(p)),
		)
		return fmt.Errorf("failed to enqueue dag-run %s: %w", name, err)
	}
	idx.append(p, fileName)
	if err := s.saveQueueIndexLocked(ctx, name, idx); err != nil {
		logger.Warn(ctx, "Failed to persist queue index after enqueue", tag.Error(err))
		s.invalidateQueueIndexLocked(ctx, name)
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
