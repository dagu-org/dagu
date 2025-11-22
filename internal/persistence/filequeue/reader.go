package filequeue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/scheduler/filenotify"
	"github.com/fsnotify/fsnotify"
)

var _ execution.QueueReader = (*queueReaderImpl)(nil)

type queueReaderImpl struct {
	store   *Store
	running atomic.Bool
	cancel  context.CancelFunc
	mu      sync.RWMutex
	done    chan struct{}
	watcher filenotify.FileWatcher
	// inFlight tracks items that have been sent to the channel but not yet processed.
	// This prevents re-sending the same item too frequently.
	inFlight sync.Map // map[string]time.Time (key: dagName:runID, value: time sent)
}

const (
	// Longer intervals since we use file events as primary notification
	reloadInterval  = 2 * time.Second // Backup polling interval
	shutdownTimeout = 5 * time.Second
	pollingInterval = 2 * time.Second // For filenotify poller fallback
	retryInterval   = 2 * time.Second
)

func newQueueReader(s *Store) *queueReaderImpl {
	return &queueReaderImpl{
		store: s,
		done:  make(chan struct{}),
	}
}

// Start implements models.QueueReader.
func (q *queueReaderImpl) Start(ctx context.Context, ch chan<- execution.QueuedItem) error {
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
			_ = watcher.Close()
			watcher = nil
		}
	}

	q.mu.Lock()
	q.cancel = cancel
	q.watcher = watcher
	q.mu.Unlock()

	go q.startWatch(ctx, ch)

	return nil
}

// startWatch starts watching the queue for new items.
// It will dequeue items from the queue and send them to the channel
func (q *queueReaderImpl) startWatch(ctx context.Context, ch chan<- execution.QueuedItem) {
	defer close(q.done)
	defer q.running.Store(false)

	reloadTicker := time.NewTicker(reloadInterval)
	defer reloadTicker.Stop()

	// Initial load
	q.processFiles(ctx, ch)

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
		select {
		case <-ctx.Done():
			logger.Info(ctx, "Stopping queue reader due to context cancellation")
			return

		case event := <-eventsCh:
			// File system event occurred - handle it
			if q.handleFileEvent(ctx, event) {
				q.processFiles(ctx, ch)
			}

		case err := <-errorsCh:
			// File watcher error
			logger.Error(ctx, "File watcher error", "err", err)

		case <-reloadTicker.C:
			// Periodic reload to catch missed events or retry failed items
			q.processFiles(ctx, ch)
		}
	}
}

func (q *queueReaderImpl) processFiles(ctx context.Context, ch chan<- execution.QueuedItem) {
	items, err := q.store.All(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to read queue items", "err", err)
		return
	}

	now := time.Now()
	blockedQueues := make(map[string]bool)

	for _, itemData := range items {
		if ctx.Err() != nil {
			return
		}

		queueName := itemData.Data().Name
		if blockedQueues[queueName] {
			continue
		}

		key := fmt.Sprintf("%s:%s", queueName, itemData.Data().ID)

		// Check if item is in flight and if retry interval has passed
		if sentTime, ok := q.inFlight.Load(key); ok {
			if now.Sub(sentTime.(time.Time)) < retryInterval {
				// Item is in flight and not ready for retry.
				// We must block this queue to preserve order, because this item (A1)
				// is effectively "at the head" of the queue processing.
				// If we skip it but process A2, we violate order if A1 needs to be retried.
				// However, A1 is *already* in the channel (or being processed).
				// Sending A2 is fine because A1 was sent *before* A2.
				// So we do NOT block the queue here.
				continue
			}
		}

		// Try to send to channel (non-blocking)
		select {
		case ch <- *execution.NewQueuedItem(itemData):
			q.inFlight.Store(key, now)
		default:
			// Channel full, will retry on next tick.
			// Mark this queue as blocked to prevent sending subsequent items (A2)
			// before this item (A1) is sent.
			blockedQueues[queueName] = true
		}
	}

	// Cleanup inFlight map for items that no longer exist
	q.inFlight.Range(func(key, value any) bool {
		k := key.(string)
		found := false
		for _, item := range items {
			if fmt.Sprintf("%s:%s", item.Data().Name, item.Data().ID) == k {
				found = true
				break
			}
		}
		if !found {
			q.inFlight.Delete(key)
		}
		return true
	})
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
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Watch the base directory for new queue files and subdirectories
	if err := watcher.Add(baseDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to watch base directory %s: %w", baseDir, err)
	}

	// Watch existing
	entries, err := os.ReadDir(baseDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read base directory %s: %w", baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			subDir := filepath.Join(baseDir, entry.Name())
			if err := watcher.Add(subDir); err != nil {
				logger.Warn(ctx, "Failed to watch queue directory", "dir", subDir, "err", err)
			} else {
				logger.Debug(ctx, "Watching queue directory", "dir", subDir)
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
						logger.Warn(ctx, "Failed to watch new queue directory", "dir", event.Name, "err", err)
					} else {
						logger.Debug(ctx, "Started watching new queue directory", "dir", event.Name)
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
