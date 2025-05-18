package scheduler

import (
	"context"
	"time"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

type QueueReader interface {
	// Start starts the queue reader in a separate goroutine.
	Start(ctx context.Context, itemCh chan models.QueuedItem, done chan any) error
}

// queueReaderImpl is a struct that reads items from a queue and sends them to a channel.
type queueReaderImpl struct {
	QueueStore models.QueueStore
	ProcStore  models.ProcStore
}

// NewQueueReader creates a new instance of QueueReader with the provided queue and process stores.
func NewQueueReader(
	qs models.QueueStore,
	ps models.ProcStore,
) QueueReader {
	return &queueReaderImpl{
		QueueStore: qs,
		ProcStore:  ps,
	}
}

// Start starts the queue reader in a separate goroutine.
func (qr *queueReaderImpl) Start(ctx context.Context, itemCh chan models.QueuedItem, done chan any) error {
	// Start the queue reader
	go qr.watchQueue(ctx, itemCh, done)
	return nil
}

func (qr *queueReaderImpl) watchQueue(ctx context.Context, itemCh chan models.QueuedItem, done chan any) {
	logger.Info(ctx, "Starting queue reader")

	const checkInterval = 2 * time.Second
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "Stopping queue reader")
			return
		case <-done:
			logger.Info(ctx, "Stopping queue reader")
			return
		case <-ticker.C:
			// Check for new items in the queue
			items, err := qr.QueueStore.All(ctx)
			if err != nil {
				logger.Error(ctx, "Failed to read queue", "err", err)
				continue
			}
			// Process each item in the queue
			for _, item := range items {
				itemCh <- item
			}
		}
	}
}
