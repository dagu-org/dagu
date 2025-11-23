package filequeue

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/service/scheduler/filenotify"
	"github.com/fsnotify/fsnotify"
)

var pollingInterval = 2 * time.Second // For filenotify poller fallback

var _ execution.QueueWatcher = (*watcher)(nil)

type watcher struct {
	baseDir     string
	fileWatcher filenotify.FileWatcher
	quit        chan struct{}
	notifyCh    chan struct{}
	wg          sync.WaitGroup
}

func newWatcher(baseDir string) execution.QueueWatcher {
	w := &watcher{
		baseDir:  baseDir,
		quit:     make(chan struct{}),
		notifyCh: make(chan struct{}),
	}
	return w
}

// Start implements execution.QueueWatcher.
func (w *watcher) Start(ctx context.Context) (<-chan struct{}, error) {
	w.fileWatcher = filenotify.New(pollingInterval)
	if err := backoff.Retry(ctx, func(ctx context.Context) error {
		// Initialize file watcher (optional - fallback to polling if it fails)
		return w.setupWatcher(ctx)
	}, backoff.NewConstantBackoffPolicy(2*time.Second), nil); err != nil {
		return nil, err
	}

	w.wg.Add(1)
	go func(ctx context.Context) {
		defer w.wg.Done()
		w.loop(ctx)
	}(ctx)

	return w.notifyCh, nil
}

func (w *watcher) loop(ctx context.Context) {
	eventsCh := w.fileWatcher.Events()
	errorsCh := w.fileWatcher.Errors()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.quit:
			return
		case event := <-eventsCh:
			if w.handleFileEvent(ctx, event) {
				select {
				case w.notifyCh <- struct{}{}:
				default:
				}
			}
		case err := <-errorsCh:
			logger.Error(ctx, "File watcher error", tag.Error(err))
		}
	}
}

// Stop implements execution.QueueWatcher.
func (w *watcher) Stop(ctx context.Context) {
	if err := w.fileWatcher.Close(); err != nil {
		logger.Error(ctx, "Failed to stop file watcher", tag.Error(err))
	}
	w.quit <- struct{}{}
	w.wg.Wait()
}

// setupWatcher sets up the file watcher for the base directory and existing subdirectories
func (w *watcher) setupWatcher(ctx context.Context) error {
	baseDir := w.baseDir

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, 0750); err != nil {
		return fmt.Errorf("failed to create base directory %s: %w", baseDir, err)
	}

	// Watch the base directory for new queue files and subdirectories
	if err := w.fileWatcher.Add(baseDir); err != nil && !os.IsNotExist(err) {
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
			if err := w.fileWatcher.Add(subDir); err != nil {
				logger.Warn(ctx, "Failed to watch queue directory",
					tag.Dir(subDir),
					tag.Error(err))
			} else {
				logger.Debug(ctx, "Watching queue directory",
					tag.Dir(subDir))
			}
		}
	}

	return nil
}

// handleFileEvent processes a file system event and returns true if items should be reloaded
func (w *watcher) handleFileEvent(ctx context.Context, event fsnotify.Event) bool {
	// Only care about Create, Write, and Remove events
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
		return false
	}

	relPath, err := filepath.Rel(w.baseDir, event.Name)
	if err != nil {
		return false
	}

	// If it's a directory creation in the base directory, add it to the watcher
	if event.Op&fsnotify.Create != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			// Check if it's a direct subdirectory of baseDir
			if strings.Count(relPath, string(filepath.Separator)) == 0 {
				if err := w.fileWatcher.Add(event.Name); err != nil {
					logger.Warn(ctx, "Failed to watch new queue directory",
						tag.Dir(event.Name),
						tag.Error(err))
				} else {
					logger.Debug(ctx, "Started watching new queue directory",
						tag.Dir(event.Name))
				}
				return true
			}
		}
	}

	// If it's removed, remove from watch directories
	if event.Op&fsnotify.Remove != 0 {
		if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
			if strings.Count(relPath, string(filepath.Separator)) == 0 {
				err := w.fileWatcher.Remove(event.Name)
				if err != nil {
					logger.Warn(ctx, "Failed to remove from file watcher",
						tag.Error(err),
						tag.Dir(event.Name))
				}
			}
		}
	}

	// Check if it's a queue file (item_*.json)
	filename := filepath.Base(event.Name)
	if strings.HasPrefix(filename, "item_") && strings.HasSuffix(filename, ".json") {
		logger.Debug(ctx, "Queue file event",
			tag.File(event.Name),
			tag.Operation(event.Op.String()))
		return true
	}

	return false
}
