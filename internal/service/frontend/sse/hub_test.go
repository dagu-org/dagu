package sse

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHub(t *testing.T) {
	t.Parallel()
	hub := NewHub()

	require.NotNil(t, hub)
	assert.NotNil(t, hub.clients)
	assert.NotNil(t, hub.watchers)
	assert.NotNil(t, hub.fetchers)
	assert.Equal(t, defaultMaxClients, hub.maxClients)
	assert.False(t, hub.started)
}

func TestNewHubWithOptions(t *testing.T) {
	t.Parallel()
	t.Run("with max clients", func(t *testing.T) {
		t.Parallel()
		hub := NewHub(WithMaxClients(50))
		assert.Equal(t, 50, hub.maxClients)
	})

	t.Run("with metrics", func(t *testing.T) {
		t.Parallel()
		registry := prometheus.NewRegistry()
		metrics := NewMetrics(registry)

		hub := NewHub(WithMetrics(metrics))
		assert.NotNil(t, hub.metrics)
	})

	t.Run("with multiple options", func(t *testing.T) {
		t.Parallel()
		registry := prometheus.NewRegistry()
		metrics := NewMetrics(registry)

		hub := NewHub(
			WithMaxClients(100),
			WithMetrics(metrics),
		)

		assert.Equal(t, 100, hub.maxClients)
		assert.NotNil(t, hub.metrics)
	})
}

func TestHubRegisterFetcher(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)

	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.mu.RLock()
	defer hub.mu.RUnlock()
	assert.NotNil(t, hub.fetchers[TopicTypeDAGRun])
}

func TestHubStart(t *testing.T) {
	t.Parallel()
	t.Run("starts heartbeat", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		hub.Start()

		assert.True(t, hub.started)
		assert.NotNil(t, hub.heartbeatTicker)
	})

	t.Run("idempotent start", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		hub.Start()
		ticker1 := hub.heartbeatTicker

		hub.Start() // Second start
		ticker2 := hub.heartbeatTicker

		assert.Same(t, ticker1, ticker2, "ticker should not change on second start")
	})
}

func TestHubShutdown(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	// Subscribe a client
	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:test-id")
	require.NoError(t, err)

	assert.Equal(t, 1, hub.ClientCount())
	assert.Equal(t, 1, hub.WatcherCount())

	hub.Shutdown()

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.WatcherCount())
	assert.True(t, client.IsClosed())
}

func TestHubSubscribe(t *testing.T) {
	t.Parallel()
	t.Run("successful subscribe", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
		hub.Start()

		client := newTestClient(t)
		err := hub.Subscribe(client, "dagrun:test-dag/run-123")

		require.NoError(t, err)
		assert.Equal(t, 1, hub.ClientCount())
		assert.Equal(t, 1, hub.WatcherCount())
	})

	t.Run("creates watcher for new topic", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
		hub.Start()

		client1 := newTestClient(t)
		client2 := newTestClient(t)

		// First client - creates watcher
		err := hub.Subscribe(client1, "dagrun:dag1/run1")
		require.NoError(t, err)
		assert.Equal(t, 1, hub.WatcherCount())

		// Second client same topic - reuses watcher
		err = hub.Subscribe(client2, "dagrun:dag1/run1")
		require.NoError(t, err)
		assert.Equal(t, 1, hub.WatcherCount())

		// Third client different topic - creates new watcher
		client3 := newTestClient(t)
		err = hub.Subscribe(client3, "dagrun:dag2/run2")
		require.NoError(t, err)
		assert.Equal(t, 2, hub.WatcherCount())
	})

	t.Run("error on max clients", func(t *testing.T) {
		t.Parallel()
		hub := NewHub(WithMaxClients(2))
		defer hub.Shutdown()

		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
		hub.Start()

		// Subscribe 2 clients
		client1 := newTestClient(t)
		client2 := newTestClient(t)
		err := hub.Subscribe(client1, "dagrun:dag1/run1")
		require.NoError(t, err)
		err = hub.Subscribe(client2, "dagrun:dag1/run2")
		require.NoError(t, err)

		// Third client should fail
		client3 := newTestClient(t)
		err = hub.Subscribe(client3, "dagrun:dag1/run3")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "max clients reached")
	})

	t.Run("error on invalid topic format", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()
		hub.Start()

		client := newTestClient(t)
		err := hub.Subscribe(client, "invalid-topic-no-colon")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid topic format")
	})

	t.Run("error on unregistered topic type", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()
		hub.Start()

		client := newTestClient(t)
		err := hub.Subscribe(client, "unknown:identifier")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "no fetcher registered")
	})
}

func TestHubUnsubscribe(t *testing.T) {
	t.Parallel()
	t.Run("removes client", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
		hub.Start()

		client := newTestClient(t)
		err := hub.Subscribe(client, "dagrun:test-id")
		require.NoError(t, err)
		assert.Equal(t, 1, hub.ClientCount())

		hub.Unsubscribe(client)
		assert.Equal(t, 0, hub.ClientCount())
	})

	t.Run("stops watcher when last client unsubscribes", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()

		fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
		hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
		hub.Start()

		client1 := newTestClient(t)
		client2 := newTestClient(t)

		err := hub.Subscribe(client1, "dagrun:test-id")
		require.NoError(t, err)
		err = hub.Subscribe(client2, "dagrun:test-id")
		require.NoError(t, err)

		assert.Equal(t, 1, hub.WatcherCount())

		// First unsubscribe - watcher remains
		hub.Unsubscribe(client1)
		assert.Equal(t, 1, hub.WatcherCount())

		// Second unsubscribe - watcher removed
		hub.Unsubscribe(client2)
		assert.Equal(t, 0, hub.WatcherCount())
	})

	t.Run("no-op for unknown client", func(t *testing.T) {
		t.Parallel()
		hub := NewHub()
		defer hub.Shutdown()
		hub.Start()

		client := newTestClient(t)

		// Should not panic
		assert.NotPanics(t, func() {
			hub.Unsubscribe(client)
		})
	})
}

func TestHubHeartbeat(t *testing.T) {
	t.Parallel()
	// Create hub with short heartbeat for testing
	hub := NewHub()
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	// Manually set a short heartbeat interval for testing
	hub.mu.Lock()
	hub.started = true
	hub.heartbeatTicker = time.NewTicker(100 * time.Millisecond)
	hub.mu.Unlock()
	hub.heartbeatWg.Add(1) // Must add before starting goroutine
	go hub.heartbeatLoop()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:test-id")
	require.NoError(t, err)

	// Start write pump
	ctx := t.Context()
	go client.WritePump(ctx)

	// Wait for at least one heartbeat
	time.Sleep(200 * time.Millisecond)

	// Client should still be connected (heartbeat didn't fail)
	assert.Equal(t, 1, hub.ClientCount())
}

func TestHubConcurrentOperations(t *testing.T) {
	t.Parallel()
	hub := NewHub(WithMaxClients(100))
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	var wg sync.WaitGroup

	// Concurrent subscribes
	for range 20 {
		wg.Go(func() {
			client := newTestClient(t)
			_ = hub.Subscribe(client, "dagrun:concurrent-id")
			time.Sleep(10 * time.Millisecond)
			hub.Unsubscribe(client)
		})
	}

	// Concurrent reads
	for range 10 {
		wg.Go(func() {
			_ = hub.ClientCount()
			_ = hub.WatcherCount()
		})
	}

	wg.Wait()
}

func TestHubMetricsIntegration(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	hub := NewHub(WithMetrics(metrics))
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	// Subscribe
	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:test-id")
	require.NoError(t, err)

	// Verify metrics
	assert.Equal(t, float64(1), getGaugeValue(t, metrics.clientsConnected))

	// Unsubscribe
	hub.Unsubscribe(client)

	// Verify metrics decreased
	assert.Equal(t, float64(0), getGaugeValue(t, metrics.clientsConnected))
}

func TestHubSendHeartbeats(t *testing.T) {
	t.Parallel()
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	hub := NewHub(WithMetrics(metrics))
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:test-id")
	require.NoError(t, err)

	// Manually trigger heartbeat
	hub.sendHeartbeats()

	// Verify heartbeat metric
	heartbeatCount := getCounterValue(t, metrics.messagesSent, EventTypeHeartbeat)
	assert.Equal(t, float64(1), heartbeatCount)
}

func TestHubCollectClients(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	client1 := newTestClient(t)
	client2 := newTestClient(t)
	_ = hub.Subscribe(client1, "dagrun:id1")
	_ = hub.Subscribe(client2, "dagrun:id2")

	clients := hub.collectClients()

	assert.Len(t, clients, 2)
}

func TestHubSendHeartbeatsClientBufferFull(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	defer hub.Shutdown()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)
	hub.Start()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:test-id")
	require.NoError(t, err)

	// Fill the client buffer so heartbeat will fail
	for range 64 {
		client.Send(&Event{Type: EventTypeData, Data: "fill"})
	}

	assert.Equal(t, 1, hub.ClientCount())

	// Send heartbeat - should fail and trigger cleanup
	hub.sendHeartbeats()

	// Client should have been closed and unsubscribed
	assert.True(t, client.IsClosed())
	assert.Equal(t, 0, hub.ClientCount())
}

func TestHubAllTopicTypes(t *testing.T) {
	t.Parallel()
	hub := NewHub()
	defer hub.Shutdown()
	hub.Start()

	// Register all topic types
	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	topicTypes := []TopicType{
		TopicTypeDAGRun,
		TopicTypeDAG,
		TopicTypeDAGRunLogs,
		TopicTypeStepLog,
		TopicTypeDAGRuns,
		TopicTypeQueueItems,
		TopicTypeQueues,
		TopicTypeDAGsList,
	}

	for _, tt := range topicTypes {
		hub.RegisterFetcher(tt, fetcher)
	}

	// Subscribe to each type
	for _, tt := range topicTypes {
		client := newTestClient(t)
		err := hub.Subscribe(client, string(tt)+":test-identifier")
		assert.NoError(t, err, "should subscribe to %s", tt)
	}

	assert.Equal(t, len(topicTypes), hub.ClientCount())
	assert.Equal(t, len(topicTypes), hub.WatcherCount())
}
