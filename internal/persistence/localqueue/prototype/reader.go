package prototype

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/scheduler/filenotify"
	"github.com/fsnotify/fsnotify"
)

var _ models.QueueReader = (*queueReaderImpl)(nil)

type queueReaderImpl struct {
	store   *Store
	running atomic.Bool
	cancel  context.CancelFunc
	mu      sync.RWMutex
	items   []queuedItem
	updated atomic.Bool
	done    chan struct{}
	watcher filenotify.FileWatcher
}

type queuedItem struct {
	*models.QueuedItem
	status int
}

const (
	statusNone = iota
	statusProcessing
	statusDone
)

const (
	// Longer intervals since we use file events as primary notification
	reloadInterval    = 30 * time.Second // Backup polling interval
	processingDelay   = 1 * time.Second
	shutdownTimeout   = 5 * time.Second
	pollingInterval   = 2 * time.Second // For filenotify poller fallback
	processingTimeout = 8 * time.Second
)

func newQueueReader(s *Store) *queueReaderImpl {
	return &queueReaderImpl{
		store: s,
		done:  make(chan struct{}),
	}
}

// Start implements models.QueueReader.
func (q *queueReaderImpl) Start(ctx context.Context, ch chan<- models.QueuedItem) error {
	if !q.running.CompareAndSwap(false, true) {
		return fmt.Errorf("queue reader already started")
	}

	ctx, cancel := context.WithCancel(ctx)

	// Initialize file watcher (optional - fallback to polling if it fails)
	watcher, err := filenotify.New(pollingInterval)
	if err != nil {
		logger.Warn(ctx, "Failed to create file watcher, falling back to polling only", "err", err)
	} else {
		// Add the base directory and existing subdirectories to watch
		baseDir := q.store.BaseDir()
		if err := q.setupWatcher(ctx, watcher, baseDir); err != nil {
			logger.Warn(ctx, "Failed to setup file watcher, falling back to polling only", "err", err)
			watcher.Close()
			watcher = nil
		}
	}

	q.mu.Lock()
	q.cancel = cancel
	q.watcher = watcher
	q.mu.Unlock()

	allItems, err := q.store.All(ctx)
	if err != nil {
		q.running.Store(false)
		if watcher != nil {
			watcher.Close()
		}
		return fmt.Errorf("failed to read initial items: %w", err)
	}

	q.setItems(allItems)

	go q.startWatch(ctx, ch)

	return nil
}

// startWatch starts watching the queue for new items.
// It will dequeue items from the queue and send them to the channel
func (q *queueReaderImpl) startWatch(ctx context.Context, ch chan<- models.QueuedItem) {
	defer close(q.done)
	defer q.running.Store(false)

	reloadTicker := time.NewTicker(reloadInterval)
	defer reloadTicker.Stop()

	items := q.getItems()

	// Get watcher channels safely
	var eventsCh <-chan fsnotify.Event
	var errorsCh <-chan error

	q.mu.RLock()
	if q.watcher != nil {
		eventsCh = q.watcher.Events()
		errorsCh = q.watcher.Errors()
	}
	q.mu.RUnlock()

	for {
		if eventsCh != nil && errorsCh != nil {
			// Use file system events when watcher is available
			select {
			case <-ctx.Done():
				logger.Info(ctx, "Stopping queue reader due to context cancellation")
				return

			case event := <-eventsCh:
				// File system event occurred - handle it
				logger.Debug(ctx, "File system event detected", "event", event.String())
				if q.handleFileEvent(ctx, event) {
					if err := q.reloadItems(ctx); err != nil {
						logger.Error(ctx, "Failed to reload queue items after file event", "err", err)
						continue
					}
					items = q.getItems()
				}

			case err := <-errorsCh:
				// File watcher error
				logger.Error(ctx, "File watcher error", "err", err)

			case <-reloadTicker.C:
				// Backup polling mechanism
				logger.Debug(ctx, "Backup polling reload")
				if err := q.reloadItems(ctx); err != nil {
					logger.Error(ctx, "Failed to reload queue items", "err", err)
					continue
				}
				items = q.getItems()

			default:
				items = q.processItems(ctx, ch, items)
				time.Sleep(processingDelay)
			}
		} else {
			// Fallback to polling only when no watcher is available
			select {
			case <-ctx.Done():
				logger.Info(ctx, "Stopping queue reader due to context cancellation")
				return

			case <-reloadTicker.C:
				// Polling mechanism
				if err := q.reloadItems(ctx); err != nil {
					logger.Error(ctx, "Failed to reload queue items", "err", err)
					continue
				}
				items = q.getItems()

			default:
				items = q.processItems(ctx, ch, items)
				time.Sleep(processingDelay)
			}
		}
	}
}

func (q *queueReaderImpl) setItems(items []models.QueuedItemData) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Clear the items slice and reset capacity if needed
	if cap(q.items) < len(items) {
		q.items = make([]queuedItem, 0, len(items))
	} else {
		q.items = q.items[:0]
	}

	// Add new items
	for _, item := range items {
		q.items = append(q.items, queuedItem{
			QueuedItem: models.NewQueuedItem(item),
			status:     statusNone,
		})
	}

	q.updated.Store(true)
}

func (q *queueReaderImpl) getItems() []queuedItem {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Create a copy to avoid race conditions
	items := make([]queuedItem, len(q.items))
	copy(items, q.items)
	return items
}

func (q *queueReaderImpl) reloadItems(ctx context.Context) error {
	allItems, err := q.store.All(ctx)
	if err != nil {
		return fmt.Errorf("failed to read queue items: %w", err)
	}
	q.setItems(allItems)
	return nil
}

func (q *queueReaderImpl) processItems(ctx context.Context, ch chan<- models.QueuedItem, items []queuedItem) []queuedItem {
	// Check if items were updated while processing
	if q.updated.Load() {
		q.updated.Store(false)
		return q.getItems()
	}

	processed := make(map[string]bool)

	for i := range items {
		if ctx.Err() != nil {
			return items
		}

		item := &items[i]
		if item.status != statusNone {
			continue
		}

		data := item.Data()
		if processed[data.Name] {
			continue
		}

		q.tryProcessItem(ctx, ch, item, data, processed)
		processed[data.Name] = true

		// Check for updates after each item
		if q.updated.Load() {
			break
		}
	}

	return items
}

func (q *queueReaderImpl) tryProcessItem(ctx context.Context, ch chan<- models.QueuedItem, item *queuedItem, data digraph.WorkflowRef, _ map[string]bool) {
	item.status = statusProcessing

	select {
	case ch <- *item.QueuedItem:
		select {
		case res := <-item.Result:
			if res {
				logger.Info(ctx, "Item processed successfully", "name", data.Name, "workflowID", data.WorkflowID)
				item.status = statusDone
				q.removeProcessedItem(ctx, data)
				return
			}
			// Item was not processed successfully
			item.status = statusNone

		case <-time.After(processingTimeout):
			// Timeout waiting for result
			logger.Warn(ctx, "Timeout waiting for item processing result", "name", data.Name, "workflowID", data.WorkflowID)
			item.status = statusNone

		case <-ctx.Done():
			item.status = statusNone
		}
	default:
		// Channel is full, reset status and try later
		item.status = statusNone
		return
	}
}

func (q *queueReaderImpl) removeProcessedItem(ctx context.Context, data digraph.WorkflowRef) {
	if _, err := q.store.DequeueByWorkflowID(ctx, data.Name, data.WorkflowID); err != nil {
		if !errors.Is(err, models.ErrQueueItemNotFound) {
			logger.Error(ctx, "Failed to dequeue item", "err", err, "name", data.Name, "workflowID", data.WorkflowID)
		}
	}
}

// Stop implements models.QueueReader.
func (q *queueReaderImpl) Stop(ctx context.Context) {
	q.mu.Lock()
	cancel := q.cancel
	watcher := q.watcher
	if cancel != nil {
		q.cancel = nil
	}
	if watcher != nil {
		q.watcher = nil
	}
	q.mu.Unlock()

	if cancel != nil {
		cancel()
		// Wait for the watch goroutine to finish
		select {
		case <-q.done:
		case <-time.After(shutdownTimeout):
			logger.Warn(ctx, "Queue reader did not stop gracefully within timeout")
		}
	}

	// Close the file watcher
	if watcher != nil {
		if err := watcher.Close(); err != nil {
			logger.Error(ctx, "Failed to close file watcher", "err", err)
		}
	}
}

// IsRunning checks if the queue reader is running.
func (q *queueReaderImpl) IsRunning() bool {
	return q.running.Load()
}

// setupWatcher sets up the file watcher for the base directory and existing subdirectories
func (q *queueReaderImpl) setupWatcher(ctx context.Context, watcher filenotify.FileWatcher, baseDir string) error {
	// Watch the base directory for new workflow directories
	if err := watcher.Add(baseDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to watch base directory %s: %w", baseDir, err)
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Watch existing workflow subdirectories
	entries, err := os.ReadDir(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read base directory %s: %w", baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(baseDir, entry.Name())
			if err := watcher.Add(subDir); err != nil {
				logger.Warn(ctx, "Failed to watch workflow directory", "dir", subDir, "err", err)
			} else {
				logger.Debug(ctx, "Watching workflow directory", "dir", subDir)
			}
		}
	}

	return nil
}

// handleFileEvent processes a file system event and returns true if items should be reloaded
func (q *queueReaderImpl) handleFileEvent(ctx context.Context, event fsnotify.Event) bool {
	// Only care about Create, Write, and Remove events
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
		return false
	}

	baseDir := q.store.BaseDir()
	relPath, err := filepath.Rel(baseDir, event.Name)
	if err != nil {
		return false
	}

	// If it's a directory creation in the base directory, add it to the watcher
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			// Check if it's a direct subdirectory of baseDir
			if strings.Count(relPath, string(filepath.Separator)) == 0 {
				q.mu.RLock()
				watcher := q.watcher
				q.mu.RUnlock()

				if watcher != nil {
					if err := watcher.Add(event.Name); err != nil {
						logger.Warn(ctx, "Failed to watch new workflow directory", "dir", event.Name, "err", err)
					} else {
						logger.Debug(ctx, "Started watching new workflow directory", "dir", event.Name)
					}
				}
				return true
			}
		}
	}

	// Check if it's a queue file (item_*.json)
	filename := filepath.Base(event.Name)
	if strings.HasPrefix(filename, "item_") && strings.HasSuffix(filename, ".json") {
		logger.Debug(ctx, "Queue file event", "file", event.Name, "op", event.Op.String())
		return true
	}

	return false
}
