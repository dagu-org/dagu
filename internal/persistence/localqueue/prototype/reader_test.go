package prototype

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
		ID:   "test-workflow-1",
	})
	require.NoError(t, err, "expected no error when adding job to store")

	err = store.Enqueue(ctx, "test-name", models.QueuePriorityHigh, digraph.DAGRunRef{
		Name: "test-name",
		ID:   "test-workflow-2",
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
			item.Result <- true // Simulate processing the item
			receivedItems = append(receivedItems, item)
		case <-timeout:
			t.Fatal("timeout waiting for items")
		}
	}

	// Verify that we received both items
	require.Len(t, receivedItems, 2, "expected to receive 2 items")

	// Verify that the high priority item was received first
	data1 := receivedItems[0].Data()
	require.Equal(t, "test-workflow-2", data1.ID, "expected high priority item first")

	data2 := receivedItems[1].Data()
	require.Equal(t, "test-workflow-1", data2.ID, "expected low priority item second")

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
		ID:   "test-workflow-1",
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
		ID:   "test-workflow-1",
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
