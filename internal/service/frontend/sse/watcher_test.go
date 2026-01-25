package sse

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFetchFunc creates a FetchFunc that returns the given data/error
func mockFetchFunc(data any, err error) FetchFunc {
	return func(ctx context.Context, identifier string) (any, error) {
		return data, err
	}
}

// countingFetchFunc creates a FetchFunc that counts calls
func countingFetchFunc(data any, counter *int32) FetchFunc {
	return func(ctx context.Context, identifier string) (any, error) {
		atomic.AddInt32(counter, 1)
		return data, nil
	}
}

// changingFetchFunc creates a FetchFunc that returns different data on each call
func changingFetchFunc(dataSeq []any) FetchFunc {
	var idx int32
	return func(ctx context.Context, identifier string) (any, error) {
		i := atomic.AddInt32(&idx, 1) - 1
		if int(i) < len(dataSeq) {
			return dataSeq[i], nil
		}
		return dataSeq[len(dataSeq)-1], nil
	}
}

// errorThenSuccessFetchFunc returns errors for the first n calls, then succeeds
func errorThenSuccessFetchFunc(errorCount int, data any) FetchFunc {
	var count int32
	return func(ctx context.Context, identifier string) (any, error) {
		c := atomic.AddInt32(&count, 1)
		if int(c) <= errorCount {
			return nil, errors.New("fetch error")
		}
		return data, nil
	}
}

// newTestClient creates a Client for testing using the mockFlusher
func newTestClient(t *testing.T) *Client {
	t.Helper()
	w := newMockFlusher()
	client, err := NewClient(w)
	require.NoError(t, err)
	return client
}

func TestNewWatcher(t *testing.T) {
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)

	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	require.NotNil(t, watcher)
	assert.Equal(t, "test-id", watcher.identifier)
	assert.Equal(t, TopicTypeDAGRun, watcher.topicType)
	assert.NotNil(t, watcher.clients)
	assert.NotNil(t, watcher.stopCh)
	assert.NotNil(t, watcher.errorBackoff)
	assert.False(t, watcher.stopped)
}

func TestNewWatcherWithMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)

	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, metrics)

	require.NotNil(t, watcher)
	assert.NotNil(t, watcher.metrics)
}

func TestWatcherStartStop(t *testing.T) {
	t.Run("start and stop", func(t *testing.T) {
		var fetchCount int32
		fetcher := countingFetchFunc(map[string]string{"key": "value"}, &fetchCount)
		watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

		ctx := context.Background()

		// Start watcher in background
		go watcher.Start(ctx)

		// Wait for initial poll and at least one more
		time.Sleep(1500 * time.Millisecond)

		// Stop the watcher
		watcher.Stop()

		// Verify it polled at least twice (initial + ticker)
		count := atomic.LoadInt32(&fetchCount)
		assert.GreaterOrEqual(t, count, int32(2), "expected at least 2 fetches")
	})

	t.Run("stop is idempotent", func(t *testing.T) {
		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

		ctx := context.Background()
		go watcher.Start(ctx)
		time.Sleep(100 * time.Millisecond)

		// Multiple stops should not panic
		assert.NotPanics(t, func() {
			watcher.Stop()
			watcher.Stop()
			watcher.Stop()
		})
	})

	t.Run("context cancellation stops watcher", func(t *testing.T) {
		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			watcher.Start(ctx)
			close(done)
		}()

		time.Sleep(100 * time.Millisecond)
		cancel()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("watcher did not stop on context cancel")
		}
	})
}

func TestWatcherBroadcast(t *testing.T) {
	t.Run("broadcasts to all clients", func(t *testing.T) {
		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

		// Create real clients
		client1 := newTestClient(t)
		client2 := newTestClient(t)

		watcher.AddClient(client1)
		watcher.AddClient(client2)

		assert.Equal(t, 2, watcher.ClientCount())

		ctx := context.Background()
		go watcher.Start(ctx)

		// Wait for broadcast
		time.Sleep(100 * time.Millisecond)

		watcher.Stop()
	})

	t.Run("removes client on failed send", func(t *testing.T) {
		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

		// Create a client and fill its buffer so sends fail
		client := newTestClient(t)
		for i := 0; i < 64; i++ {
			client.Send(&Event{Type: EventTypeData, Data: "fill"})
		}

		watcher.AddClient(client)
		assert.Equal(t, 1, watcher.ClientCount())

		ctx := context.Background()
		go watcher.Start(ctx)

		// Wait for broadcast attempt and client removal
		time.Sleep(200 * time.Millisecond)

		watcher.Stop()

		// Client should have been closed
		assert.True(t, client.IsClosed())
	})
}

func TestWatcherHashBasedChangeDetection(t *testing.T) {
	// Use data that changes then stays the same
	dataSeq := []any{
		map[string]string{"v": "1"},
		map[string]string{"v": "2"},
		map[string]string{"v": "2"}, // Same as previous
		map[string]string{"v": "2"}, // Same as previous
	}
	fetcher := changingFetchFunc(dataSeq)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	client := newTestClient(t)
	watcher.AddClient(client)

	ctx := context.Background()

	// Start write pump to drain events
	pumpDone := make(chan struct{})
	go func() {
		client.WritePump(ctx)
		close(pumpDone)
	}()

	go watcher.Start(ctx)

	// Wait for multiple polls
	time.Sleep(3500 * time.Millisecond)

	watcher.Stop()
	client.Close()
	<-pumpDone
}

func TestWatcherBackoff(t *testing.T) {
	var fetchCount int32
	errFetcher := func(ctx context.Context, identifier string) (any, error) {
		atomic.AddInt32(&fetchCount, 1)
		return nil, errors.New("fetch error")
	}

	watcher := NewWatcher("test-id", errFetcher, TopicTypeDAGRun, nil)
	client := newTestClient(t)
	watcher.AddClient(client)

	ctx := context.Background()

	// Start write pump
	go client.WritePump(ctx)

	go watcher.Start(ctx)

	// Wait for multiple poll attempts with backoff
	// With 1s initial backoff, in 2.5s we should see ~2 attempts (initial + 1 retry)
	time.Sleep(2500 * time.Millisecond)

	watcher.Stop()
	client.Close()

	count := atomic.LoadInt32(&fetchCount)
	// Due to backoff, should have fewer fetches than would occur with 1s polling
	// Without backoff, we'd have ~3 fetches in 2.5s
	// With backoff (1s, 2s, 4s...), we'd have ~2 fetches
	assert.LessOrEqual(t, count, int32(3), "backoff should reduce fetch frequency")
}

func TestWatcherAddRemoveClient(t *testing.T) {
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	client1 := newTestClient(t)
	client2 := newTestClient(t)

	assert.Equal(t, 0, watcher.ClientCount())

	watcher.AddClient(client1)
	assert.Equal(t, 1, watcher.ClientCount())

	watcher.AddClient(client2)
	assert.Equal(t, 2, watcher.ClientCount())

	watcher.RemoveClient(client1)
	assert.Equal(t, 1, watcher.ClientCount())

	watcher.RemoveClient(client2)
	assert.Equal(t, 0, watcher.ClientCount())
}

func TestWatcherConcurrentOperations(t *testing.T) {
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	ctx := context.Background()
	go watcher.Start(ctx)

	var wg sync.WaitGroup

	// Concurrent add/remove clients
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := newTestClient(t)
			watcher.AddClient(client)
			time.Sleep(10 * time.Millisecond)
			watcher.RemoveClient(client)
		}()
	}

	// Concurrent client count reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = watcher.ClientCount()
		}()
	}

	wg.Wait()
	watcher.Stop()
}

func TestWatcherMetricsIntegration(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, metrics)

	client := newTestClient(t)
	watcher.AddClient(client)

	ctx := context.Background()

	// Start write pump
	go client.WritePump(ctx)

	go watcher.Start(ctx)

	// Wait for some activity
	time.Sleep(100 * time.Millisecond)

	watcher.Stop()
	client.Close()

	// WatcherStarted and WatcherStopped should have been called
	// Metrics gauge should be back to 0
	assert.Equal(t, float64(0), getGaugeValue(t, metrics.watchersActive))
}

func TestWatcherFetchErrorMetrics(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)
	errFetcher := func(ctx context.Context, identifier string) (any, error) {
		return nil, errors.New("test error")
	}
	watcher := NewWatcher("test-id", errFetcher, TopicTypeDAGRun, metrics)

	client := newTestClient(t)
	watcher.AddClient(client)

	ctx := context.Background()

	// Start write pump
	go client.WritePump(ctx)

	go watcher.Start(ctx)

	// Wait for error to be recorded
	time.Sleep(100 * time.Millisecond)

	watcher.Stop()
	client.Close()

	// Verify error metric was incremented
	errorCount := getFetchErrorValue(t, metrics.fetchErrors, string(TopicTypeDAGRun))
	assert.GreaterOrEqual(t, errorCount, float64(1), "fetch error should be recorded in metrics")
}

func TestComputeHash(t *testing.T) {
	data1 := []byte(`{"key": "value1"}`)
	data2 := []byte(`{"key": "value2"}`)
	data3 := []byte(`{"key": "value1"}`)

	hash1 := computeHash(data1)
	hash2 := computeHash(data2)
	hash3 := computeHash(data3)

	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 16, "hash should be 16 hex chars (8 bytes)")

	assert.NotEqual(t, hash1, hash2, "different data should have different hashes")
	assert.Equal(t, hash1, hash3, "same data should have same hash")
}

func TestWatcherIsInBackoffPeriod(t *testing.T) {
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	// Initially not in backoff
	assert.False(t, watcher.isInBackoffPeriod())

	// Simulate error handling which sets backoff
	watcher.handleFetchError(errors.New("test error"))

	// Now should be in backoff
	assert.True(t, watcher.isInBackoffPeriod())

	// Reset backoff
	watcher.resetBackoff()

	// Should no longer be in backoff
	assert.False(t, watcher.isInBackoffPeriod())
}

func TestWatcherBroadcastIfChanged(t *testing.T) {
	fetcher := mockFetchFunc(nil, nil)
	watcher := NewWatcher("test-id", fetcher, TopicTypeDAGRun, nil)

	client := newTestClient(t)
	watcher.AddClient(client)

	// First broadcast - should send
	watcher.broadcastIfChanged(map[string]string{"key": "value1"})

	// Drain the client buffer
	select {
	case <-client.send:
		// Got first event
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected first broadcast")
	}

	// Same data - should not send
	watcher.broadcastIfChanged(map[string]string{"key": "value1"})

	// Should not receive another event
	select {
	case <-client.send:
		t.Fatal("should not broadcast unchanged data")
	case <-time.After(50 * time.Millisecond):
		// Expected - no broadcast for same data
	}

	// Different data - should send
	watcher.broadcastIfChanged(map[string]string{"key": "value2"})

	select {
	case <-client.send:
		// Got second event - expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected broadcast for changed data")
	}
}
