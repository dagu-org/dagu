package filequeue

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueueReader(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Add some items to the queue
	err := store.Enqueue(ctx, "test-name", models.QueuePriorityLow, digraph.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	err = store.Enqueue(ctx, "test-name", models.QueuePriorityHigh, digraph.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan models.QueuedItem, 10)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Wait for items to be received
	receivedItems := make([]models.QueuedItem, 0, 2)
	timeout := time.After(5 * time.Second)

	for i := 0; i < 2; i++ {
		select {
		case item := <-ch:
			item.Result <- models.QueuedItemProcessingResultSuccess // Simulate processing the item
			receivedItems = append(receivedItems, item)
		case <-timeout:
			t.Fatal("timeout waiting for items")
		}
	}

	// Verify that we received both items
	require.Len(t, receivedItems, 2, "expected to receive 2 items")

	// Verify that the high priority item was received first
	data1 := receivedItems[0].Data()
	require.Equal(t, "test-dag-2", data1.ID, "expected high priority item first")

	data2 := receivedItems[1].Data()
	require.Equal(t, "test-dag-1", data2.ID, "expected low priority item second")

	// Stop the reader
	reader.Stop(ctx)
	require.False(t, reader.IsRunning(), "expected reader to be not running after stop")
}

func TestQueueReaderChannelFull(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", models.QueuePriorityLow, digraph.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel with buffer size 0 to simulate a full channel
	ch := make(chan models.QueuedItem)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Wait a bit to allow the reader to process the item
	time.Sleep(1 * time.Second)

	// The item should be re-enqueued since the channel is full
	items, err := store.List(ctx, "test-name")
	require.NoError(t, err, "expected no error when listing items")
	require.Len(t, items, 1, "expected 1 item to be re-enqueued")

	// Stop the reader
	reader.Stop(ctx)
	require.False(t, reader.IsRunning(), "expected reader to be not running after stop")
}

func TestQueueReaderStartStop(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan models.QueuedItem, 10)

	// Start the reader
	err := reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Try to start it again, should fail
	err = reader.Start(ctx, ch)
	require.Error(t, err, "expected error when starting reader twice")
	require.Contains(t, err.Error(), "already started", "expected error to mention 'already started'")

	// Stop the reader
	reader.Stop(ctx)

	require.False(t, reader.IsRunning(), "expected reader to be not running after stop")
}

func TestQueueReaderContextCancellation(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", models.QueuePriorityLow, digraph.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel with buffer size 0 to simulate a full channel
	ch := make(chan models.QueuedItem)

	// Create a context that will be cancelled
	ctxWithCancel, cancelFunc := context.WithCancel(ctx)

	// Start the reader
	err = reader.Start(ctxWithCancel, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Wait a bit to allow the reader to start
	time.Sleep(100 * time.Millisecond)

	// Cancel the context
	cancelFunc()

	// Wait a bit to allow the reader to process the cancellation
	time.Sleep(500 * time.Millisecond)

	// The reader should have stopped due to context cancellation
	// We can verify this by trying to stop it, which should fail
	require.False(t, reader.IsRunning(), "expected reader to be not running after context cancellation")
}

func TestQueueReaderRetryDelay(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 10*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Add multiple items from the same queue
	queueName := "test-queue"
	for i := 0; i < 3; i++ {
		err := store.Enqueue(ctx, queueName, models.QueuePriorityHigh, digraph.DAGRunRef{
			Name: queueName,
			ID:   "test-dag-" + string(rune('1'+i)),
		})
		require.NoError(t, err, "expected no error when adding job to store")
	}

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan models.QueuedItem, 1)

	// Start the reader
	err := reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Track processing attempts
	var processingAttempts []time.Time
	retryCount := 0

	// Process items
	go func() {
		for {
			select {
			case item := <-ch:
				processingAttempts = append(processingAttempts, time.Now())
				
				// Simulate queue being full for first 2 attempts
				if retryCount < 2 {
					retryCount++
					item.Result <- models.QueuedItemProcessingResultRetry
				} else {
					// Process successfully on third attempt
					item.Result <- models.QueuedItemProcessingResultSuccess
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for processing
	time.Sleep(6 * time.Second)

	// Stop the reader
	reader.Stop(ctx)

	// Verify retry delay behavior
	require.GreaterOrEqual(t, len(processingAttempts), 3, "expected at least 3 processing attempts")
	
	// Check that there was a delay between retry attempts
	if len(processingAttempts) >= 2 {
		delay := processingAttempts[1].Sub(processingAttempts[0])
		// The delay should be at least close to queueRetryDelay (2 seconds)
		require.GreaterOrEqual(t, delay, 1900*time.Millisecond, "expected delay between retries to be at least 1.9 seconds")
		require.LessOrEqual(t, delay, 3*time.Second, "expected delay between retries to be less than 3 seconds")
	}
}

func TestQueueReaderRetryDelayPerQueue(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 10*time.Second)
	defer cancel()

	// Create a new store
	store := New(th.Config.Paths.QueueDir)

	// Add items from different queues
	queue1 := "queue-1"
	queue2 := "queue-2"
	
	err := store.Enqueue(ctx, queue1, models.QueuePriorityHigh, digraph.DAGRunRef{
		Name: queue1,
		ID:   "dag-1",
	})
	require.NoError(t, err)
	
	err = store.Enqueue(ctx, queue2, models.QueuePriorityHigh, digraph.DAGRunRef{
		Name: queue2,
		ID:   "dag-2",
	})
	require.NoError(t, err)

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan models.QueuedItem, 1)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Track processing by queue
	queueProcessing := make(map[string][]time.Time)

	// Process items
	go func() {
		for {
			select {
			case item := <-ch:
				queueName := item.Data().Name
				queueProcessing[queueName] = append(queueProcessing[queueName], time.Now())
				
				// Always return retry to test delay behavior
				item.Result <- models.QueuedItemProcessingResultRetry
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for processing
	time.Sleep(5 * time.Second)

	// Stop the reader
	reader.Stop(ctx)

	// Verify that both queues were processed
	require.Contains(t, queueProcessing, queue1, "expected queue-1 to be processed")
	require.Contains(t, queueProcessing, queue2, "expected queue-2 to be processed")
	
	// Verify that retry delays are applied per queue independently
	// Each queue should have been retried, but queue2 shouldn't be delayed by queue1's retry
	require.GreaterOrEqual(t, len(queueProcessing[queue1]), 2, "expected queue-1 to have at least 2 attempts")
	require.GreaterOrEqual(t, len(queueProcessing[queue2]), 2, "expected queue-2 to have at least 2 attempts")
}
