package prototype

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
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
	reloadInterval   = 2 * time.Second
	processingDelay  = 500 * time.Millisecond
	shutdownTimeout  = 5 * time.Second
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

	q.mu.Lock()
	q.cancel = cancel
	q.mu.Unlock()

	allItems, err := q.store.All(ctx)
	if err != nil {
		q.running.Store(false)
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

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "Stopping queue reader due to context cancellation")
			return

		case <-reloadTicker.C:
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

		if q.tryProcessItem(ctx, ch, item, data, processed) {
			processed[data.Name] = true
		}

		// Check for updates after each item
		if q.updated.Load() {
			break
		}
	}

	return items
}

func (q *queueReaderImpl) tryProcessItem(ctx context.Context, ch chan<- models.QueuedItem, item *queuedItem, data digraph.WorkflowRef, processed map[string]bool) bool {
	item.status = statusProcessing

	select {
	case ch <- *item.QueuedItem:
		select {
		case res := <-item.Result:
			if res {
				logger.Info(ctx, "Item processed successfully", "name", data.Name, "workflowID", data.WorkflowID)
				item.status = statusDone
				q.removeProcessedItem(ctx, data)
				return true
			}
			logger.Warn(ctx, "Item processing failed", "name", data.Name, "workflowID", data.WorkflowID)
			item.status = statusNone
			return false
		case <-ctx.Done():
			item.status = statusNone
			return false
		}
	default:
		// Channel is full, reset status and try later
		item.status = statusNone
		return false
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
	if cancel != nil {
		q.cancel = nil
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
}

// IsRunning checks if the queue reader is running.
func (q *queueReaderImpl) IsRunning() bool {
	return q.running.Load()
}
