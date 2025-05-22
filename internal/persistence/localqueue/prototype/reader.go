package prototype

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.QueueReader = (*queueReaderImpl)(nil)

type queueReaderImpl struct {
	store   *Store
	running atomic.Bool
	cancel  context.CancelFunc
	mu      sync.Mutex
	items   []queuedItem
	updated atomic.Bool
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

func newQueueReader(s *Store) *queueReaderImpl {
	return &queueReaderImpl{store: s}
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
		return fmt.Errorf("failed to read initial items: %w", err)
	}

	q.setItems(allItems)

	go q.startWatch(ctx, ch)

	return nil
}

// startWatch starts watching the queue for new items.
// It will dequeue items from the queue and send them to the channel
func (q *queueReaderImpl) startWatch(ctx context.Context, ch chan<- models.QueuedItem) {
	reloadTicker := time.NewTicker(2 * time.Second)
	defer reloadTicker.Stop()

	q.mu.Lock()
	items := q.items
	q.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			// Context is cancelled, stop processing
			logger.Info(ctx, "Stopping queue reader due to context cancellation")
			q.Stop(ctx)
			return

		case <-reloadTicker.C:
			// Reload the items from the queue
			// TODO: Improve this to be event-driven instead of polling (e.g., using fsnotify)
			items, err := q.store.All(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read queue", "err", err)
				continue
			}
			q.setItems(items)

		default:
			var processed sync.Map

			for i := 0; i < len(items); i++ {
				if ctx.Err() != nil {
					// Context is cancelled, stop processing
					return
				}
				if items[i].status != statusNone {
					// Skip already processed items
					continue
				}
				if _, ok := processed.Load(items[i].Data().Name); ok {
					// Skip already processed items
					continue
				}

				item := items[i]
				data := item.Data()

				items[i].status = statusProcessing

				// Send the item to the channel
				select {
				case ch <- *item.QueuedItem:
					select {
					case res := <-item.Result:
						if !res {
							// Item processing failed
							continue
						}

						// Item was processed successfully
						logger.Info(ctx, "Item processed successfully", "item", item)
						item.status = statusDone
						processed.Store(data.Name, true)

						// Remove the item from the queue
						_, err := q.store.DequeueByWorkflowID(ctx, data.Name, data.WorkflowID)
						if err != nil {
							if errors.Is(err, models.ErrQueueItemNotFound) {
								continue
							}
							// Unexpected error, log it
							logger.Error(ctx, "Failed to dequeue item", "err", err, "name", data.Name, "workflowID", data.WorkflowID)
							continue
						}
					}
				default:
					// Channel is full, skip sending the item
					items[i].status = statusNone
				}

				// Check if the item list has been updated
				if q.updated.Load() {
					break
				}
			}

			// If the item list has been updated, reload the items
			if q.updated.Load() {
				q.mu.Lock()
				items = q.items
				q.updated.Store(false) // Reset the updated flag
				q.mu.Unlock()
			}

			// Sleep for a short duration to avoid busy waiting
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (q *queueReaderImpl) setItems(items []models.QueuedItemData) {
	q.mu.Lock()
	// clear the items
	q.items = q.items[:0]
	for i := 0; i < len(items); i++ {
		q.items = append(q.items, queuedItem{
			QueuedItem: models.NewQueuedItem(items[i]),
		})
	}

	q.updated.Store(true)
	q.mu.Unlock()
}

// Stop implements models.QueueReader.
func (q *queueReaderImpl) Stop(ctx context.Context) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if !q.running.CompareAndSwap(true, false) {
		// It's already stopped
		return
	}

	if q.cancel != nil {
		q.cancel()
		q.cancel = nil
	}
}

// IsRunning checks if the queue reader is running.
func (q *queueReaderImpl) IsRunning() bool {
	return q.running.Load()
}
