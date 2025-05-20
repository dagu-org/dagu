package prototype

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
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
	items   []models.QueuedItem
	updated atomic.Bool
}

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
			for _, item := range items {
				if ctx.Err() != nil {
					// Context is cancelled, stop processing
					return
				}

				data := item.Data()
				_, err := q.store.DequeueByWorkflowID(ctx, data.Name, data.WorkflowID)
				if err != nil {
					if errors.Is(err, models.ErrQueueItemNotFound) {
						// Perhaps the item was already processed
						continue
					}
					// Unexpected error, log it
					logger.Error(ctx, "Failed to dequeue item", "err", err, "name", data.Name, "workflowID", data.WorkflowID)
					continue
				}
				// Send the item to the channel
				select {
				case ch <- item:
				default:
					// Return it to the queue with retries
					maxRetries := 3
					initialBackoff := 500 * time.Millisecond

					for retryCount := 0; retryCount < maxRetries; retryCount++ {
						err := q.store.Enqueue(ctx, data.Name, models.QueuePriorityHigh, data)
						if err == nil {
							// Success, break out of the retry loop
							break
						}
						if retryCount == maxRetries-1 {
							// For now, just log the error and forget about it
							// TODO: Implement a dead-letter queue or similar mechanism
							logger.Error(ctx, "Failed to return item to queue after multiple retries", "err", err, "data", data, "retries", maxRetries)
							break
						}
						logger.Warn(ctx, "Failed to return item to queue", "err", err, "data", data, "retry", retryCount, "maxRetries", maxRetries)

						// backoff multiplier = 2^retryCount (max: 2^3 = 8)
						// max backoff = 2^3 * 500ms * 1.25(jitter) = 4s * 1.25 = 5s
						backoffMultiplier := math.Pow(2, float64(retryCount))
						backoff := initialBackoff * time.Duration(backoffMultiplier)
						jitter := getSecureRandomDuration(backoff / 4)
						backoff += jitter

						// Wait for the backoff duration before retrying
						select {
						case <-ctx.Done():
							// Context is cancelled, stop retrying
							logger.Warn(ctx, "Context cancelled, stopping retry", "data", data)
							break
							// case <-time.After(backoff):
							// Continue to the next retry
						}
					}
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
				q.updated.Store(false)
				q.mu.Unlock()
			} else {
				// Sleep for a short duration to avoid busy waiting
				// time.Sleep(500 * time.Millisecond)
			}
		}
	}
}

func (q *queueReaderImpl) setItems(items []models.QueuedItem) {
	q.mu.Lock()
	q.items = items
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

// getSecureRandomDuration generates a random duration between 0 and maxDuration
// using crypto/rand for true randomness
func getSecureRandomDuration(maxDuration time.Duration) time.Duration {
	// Create a buffer to hold 8 bytes (uint64)
	var buf [8]byte

	// Read random bytes from crypto/rand
	_, err := rand.Read(buf[:])
	if err != nil {
		// If crypto/rand fails, return a simple fraction of maxDuration
		// This is a fallback that shouldn't normally be needed
		return maxDuration / 2
	}

	// Convert bytes to uint64
	randomUint64 := binary.BigEndian.Uint64(buf[:])

	// Scale the random value to be between 0 and maxDuration
	randomFraction := float64(randomUint64) / float64(math.MaxUint64)
	return time.Duration(float64(maxDuration) * randomFraction)
}
