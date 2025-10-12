package filequeue_test

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/persistence/filequeue"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestQueueReader(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second) // Allow for processing delay
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add some items to the queue
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	err = store.Enqueue(ctx, "test-name", execution.QueuePriorityHigh, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan execution.QueuedItem, 10)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Wait for items to be received
	receivedItems := make([]execution.QueuedItem, 0, 2)
	timeout := time.After(5 * time.Second) // Account for processingDelay between items

	for i := 0; i < 2; i++ {
		select {
		case item := <-ch:
			item.Result <- execution.QueuedItemProcessingResultSuccess // Simulate processing the item
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
	ctx, cancel := context.WithTimeout(th.Context, 2*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel with buffer size 0 to simulate a full channel
	ch := make(chan execution.QueuedItem)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Poll for the item to be re-enqueued
	require.Eventually(t, func() bool {
		items, err := store.List(ctx, "test-name")
		return err == nil && len(items) == 1
	}, 500*time.Millisecond, 10*time.Millisecond, "expected 1 item to be re-enqueued")

	// Stop the reader
	reader.Stop(ctx)
	require.False(t, reader.IsRunning(), "expected reader to be not running after stop")
}

func TestQueueReaderStartStop(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan execution.QueuedItem, 10)

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
	ctx, cancel := context.WithTimeout(th.Context, 2*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, core.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel with buffer size 0 to simulate a full channel
	ch := make(chan execution.QueuedItem)

	// Create a context that will be cancelled
	ctxWithCancel, cancelFunc := context.WithCancel(ctx)

	// Start the reader
	err = reader.Start(ctxWithCancel, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Poll to ensure reader is running
	require.Eventually(t, func() bool {
		return reader.IsRunning()
	}, 100*time.Millisecond, 5*time.Millisecond, "expected reader to be running")

	// Cancel the context
	cancelFunc()

	// Poll for reader to stop
	require.Eventually(t, func() bool {
		return !reader.IsRunning()
	}, 200*time.Millisecond, 10*time.Millisecond, "expected reader to stop after context cancellation")
}

func TestQueueReaderRetryDelay(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add a single item
	queueName := "test-queue"
	err := store.Enqueue(ctx, queueName, execution.QueuePriorityHigh, core.DAGRunRef{
		Name: queueName,
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan execution.QueuedItem, 1)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// First attempt - return retry
	select {
	case item := <-ch:
		item.Result <- execution.QueuedItemProcessingResultRetry
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first item")
	}

	// Verify no immediate retry (should be delayed)
	select {
	case <-ch:
		t.Fatal("expected delay before retry, but got item immediately")
	case <-time.After(100 * time.Millisecond):
		// Good, no immediate retry
	}

	// Wait a bit more and then we should get the retry
	select {
	case item := <-ch:
		item.Result <- execution.QueuedItemProcessingResultSuccess
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for retry")
	}

	// Stop the reader
	reader.Stop(ctx)
}

func TestQueueReaderRetryDelayPerQueue(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add items from different queues
	queue1 := "queue-1"
	queue2 := "queue-2"

	err := store.Enqueue(ctx, queue1, execution.QueuePriorityHigh, core.DAGRunRef{
		Name: queue1,
		ID:   "dag-1",
	})
	require.NoError(t, err)

	err = store.Enqueue(ctx, queue2, execution.QueuePriorityHigh, core.DAGRunRef{
		Name: queue2,
		ID:   "dag-2",
	})
	require.NoError(t, err)

	// Get a reader from the store
	reader := store.Reader(ctx)

	// Create a channel to receive items
	ch := make(chan execution.QueuedItem, 1)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Track which queues we've seen
	seenQueues := make(map[string]bool)

	// Process first two items and return retry
	for i := 0; i < 2; i++ {
		select {
		case item := <-ch:
			seenQueues[item.Data().Name] = true
			item.Result <- execution.QueuedItemProcessingResultRetry
		case <-time.After(2 * time.Second):
			t.Fatal("timeout waiting for item")
		}
	}

	// Both queues should have been processed once
	require.Len(t, seenQueues, 2, "expected both queues to be processed")
	require.Contains(t, seenQueues, queue1)
	require.Contains(t, seenQueues, queue2)

	// Now verify that retry delay is applied per queue
	// We should NOT get any items immediately
	select {
	case <-ch:
		t.Fatal("expected delay before retry, but got item immediately")
	case <-time.After(100 * time.Millisecond):
		// Good, retry is delayed
	}

	// Stop the reader
	reader.Stop(ctx)
}
