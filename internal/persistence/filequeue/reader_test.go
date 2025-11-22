package filequeue_test

import (
	"context"
	"fmt"
	"testing"
	"time"

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
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, execution.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	err = store.Enqueue(ctx, "test-name", execution.QueuePriorityHigh, execution.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-2",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader()

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
			receivedItems = append(receivedItems, item)
		case <-timeout:
			t.Fatal("timeout waiting for items")
		}
	}

	// Verify that we received both items
	require.Len(t, receivedItems, 2, "expected to receive 2 items")

	// Stop the reader
	reader.Stop(ctx)
	require.False(t, reader.IsRunning(), "expected reader to be not running after stop")
}

func TestQueueReaderChannelFull(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, execution.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader()

	// Create a channel with buffer size 0 to simulate a full channel
	ch := make(chan execution.QueuedItem)

	// Start the reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// The reader should try to send, fail (block/default), and then retry later.
	// We can't easily verify the "retry" without mocking time or waiting,
	// but we can verify that if we start reading, we eventually get it.

	go func() {
		time.Sleep(1 * time.Second)
		<-ch // Read one item to unblock
	}()

	select {
	case <-ch:
		// We got the item eventually
	case <-time.After(4 * time.Second):
		t.Fatal("timeout waiting for item after unblocking")
	}

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
	reader := store.Reader()

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

func TestQueueReaderChannelFullOrdering(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 5*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add items to the queue
	queueName := "test-ordering"
	numItems := 5
	for i := 0; i < numItems; i++ {
		err := store.Enqueue(ctx, queueName, execution.QueuePriorityLow, execution.DAGRunRef{
			Name: queueName,
			ID:   fmt.Sprintf("run-%d", i),
		})
		require.NoError(t, err)
	}

	// Get a reader from the store
	reader := store.Reader()

	// Create a channel with buffer size 0 to simulate a full channel immediately
	ch := make(chan execution.QueuedItem)

	// Start the reader
	err := reader.Start(ctx, ch)
	require.NoError(t, err, "expected no error when starting reader")

	// Read items and verify order
	for i := 0; i < numItems; i++ {
		select {
		case item := <-ch:
			require.Equal(t, fmt.Sprintf("run-%d", i), item.Data().ID, "expected items to be received in order")
			// Simulate processing and dequeue
			_, err := store.DequeueByDAGRunID(ctx, queueName, item.Data().ID)
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatalf("timeout waiting for item %d", i)
		}
	}

	// Stop the reader
	reader.Stop(ctx)
}

func TestQueueReader_HeadOfLineBlocking(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 10*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)
	queueName := "test-hol-blocking"

	// Enqueue Item A
	err := store.Enqueue(ctx, queueName, execution.QueuePriorityLow, execution.DAGRunRef{
		Name: queueName,
		ID:   "run-A",
	})
	require.NoError(t, err)

	// Enqueue Item B
	err = store.Enqueue(ctx, queueName, execution.QueuePriorityLow, execution.DAGRunRef{
		Name: queueName,
		ID:   "run-B",
	})
	require.NoError(t, err)

	// Get a reader
	reader := store.Reader()
	ch := make(chan execution.QueuedItem, 10)

	// Start reader
	err = reader.Start(ctx, ch)
	require.NoError(t, err)

	// 1. Receive Item A
	select {
	case item := <-ch:
		require.Equal(t, "run-A", item.Data().ID)
		// DO NOT dequeue A. Simulate failure.
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for run-A")
	}

	// 2. Wait for retry interval (2s) + buffer
	time.Sleep(3 * time.Second)

	// 3. Should receive Item A again (retry)
	// Item B should NOT be received because A is still in queue (Head-of-Line blocking)
	select {
	case item := <-ch:
		require.Equal(t, "run-A", item.Data().ID, "expected run-A to be retried")
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for run-A retry")
	}

	// 4. Verify channel is empty (Item B not sent)
	select {
	case item := <-ch:
		t.Fatalf("received unexpected item %s. Strict ordering violated!", item.Data().ID)
	default:
		// OK
	}

	reader.Stop(ctx)
}

func TestQueueReaderContextCancellation(t *testing.T) {
	th := test.Setup(t)
	ctx, cancel := context.WithTimeout(th.Context, 2*time.Second)
	defer cancel()

	// Create a new store
	store := filequeue.New(th.Config.Paths.QueueDir)

	// Add an item to the queue
	err := store.Enqueue(ctx, "test-name", execution.QueuePriorityLow, execution.DAGRunRef{
		Name: "test-name",
		ID:   "test-dag-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	// Get a reader from the store
	reader := store.Reader()

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
