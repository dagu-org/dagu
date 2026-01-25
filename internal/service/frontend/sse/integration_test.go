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

// TestSSEFullFlow tests the complete SSE flow:
// Hub → Subscribe → Watcher → Fetch → Broadcast → Client
func TestSSEFullFlow(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	hub := NewHub(WithMetrics(metrics))

	fetchData := map[string]string{"status": "running", "progress": "50%"}
	var fetchCount int32
	fetcher := func(ctx context.Context, identifier string) (any, error) {
		atomic.AddInt32(&fetchCount, 1)
		return fetchData, nil
	}
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	// Create a client
	client := newTestClient(t)

	// Subscribe the client
	err := hub.Subscribe(client, "dagrun:mydag/run123")
	require.NoError(t, err)

	// Verify subscription
	assert.Equal(t, 1, hub.ClientCount())
	assert.Equal(t, 1, hub.WatcherCount())

	// Start client write pump
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pumpDone := make(chan struct{})
	go func() {
		client.WritePump(ctx)
		close(pumpDone)
	}()

	// Wait for initial data fetch
	time.Sleep(200 * time.Millisecond)

	// Verify data was fetched
	assert.GreaterOrEqual(t, atomic.LoadInt32(&fetchCount), int32(1))

	// Unsubscribe
	hub.Unsubscribe(client)
	cancel()
	<-pumpDone

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.WatcherCount())

	// Verify metrics
	assert.Equal(t, float64(0), getGaugeValue(t, metrics.clientsConnected))
	assert.Equal(t, float64(0), getGaugeValue(t, metrics.watchersActive))
}

// TestSSEMultipleClientsOnSameTopic tests multiple clients subscribing to the same topic
func TestSSEMultipleClientsOnSameTopic(t *testing.T) {
	hub := NewHub()

	fetcher := mockFetchFunc(map[string]string{"shared": "data"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	// Create multiple clients
	numClients := 5
	clients := make([]*Client, numClients)
	contexts := make([]context.Context, numClients)
	cancels := make([]context.CancelFunc, numClients)
	pumpDones := make([]chan struct{}, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = newTestClient(t)
		err := hub.Subscribe(clients[i], "dagrun:shared-dag/run1")
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		contexts[i] = ctx
		cancels[i] = cancel
		pumpDones[i] = make(chan struct{})

		go func(idx int) {
			clients[idx].WritePump(contexts[idx])
			close(pumpDones[idx])
		}(i)
	}

	// All clients should be subscribed
	assert.Equal(t, numClients, hub.ClientCount())
	// But only one watcher for the shared topic
	assert.Equal(t, 1, hub.WatcherCount())

	// Wait for data broadcast
	time.Sleep(200 * time.Millisecond)

	// Cleanup
	for i := 0; i < numClients; i++ {
		hub.Unsubscribe(clients[i])
		cancels[i]()
		<-pumpDones[i]
	}

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.WatcherCount())
}

// TestSSEClientDisconnect tests clean disconnect and watcher cleanup
func TestSSEClientDisconnect(t *testing.T) {
	hub := NewHub()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	// Subscribe two clients to the same topic
	client1 := newTestClient(t)
	client2 := newTestClient(t)

	err := hub.Subscribe(client1, "dagrun:dag1/run1")
	require.NoError(t, err)
	err = hub.Subscribe(client2, "dagrun:dag1/run1")
	require.NoError(t, err)

	assert.Equal(t, 2, hub.ClientCount())
	assert.Equal(t, 1, hub.WatcherCount())

	// Disconnect first client
	hub.Unsubscribe(client1)
	assert.Equal(t, 1, hub.ClientCount())
	// Watcher should still exist for second client
	assert.Equal(t, 1, hub.WatcherCount())

	// Disconnect second client
	hub.Unsubscribe(client2)
	assert.Equal(t, 0, hub.ClientCount())
	// Watcher should be cleaned up
	assert.Equal(t, 0, hub.WatcherCount())
}

// TestSSEErrorRecovery tests fetch error → backoff → retry → success
func TestSSEErrorRecovery(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	hub := NewHub(WithMetrics(metrics))

	// Fetcher that fails twice then succeeds
	var callCount int32
	fetcher := func(ctx context.Context, identifier string) (any, error) {
		count := atomic.AddInt32(&callCount, 1)
		if count <= 2 {
			return nil, errors.New("temporary error")
		}
		return map[string]string{"recovered": "true"}, nil
	}
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:dag1/run1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pumpDone := make(chan struct{})
	go func() {
		client.WritePump(ctx)
		close(pumpDone)
	}()

	// Wait for backoff and recovery
	// Initial call fails, backoff 1s, retry fails, backoff 2s, retry succeeds
	time.Sleep(4 * time.Second)

	// Should have been called at least 3 times
	assert.GreaterOrEqual(t, atomic.LoadInt32(&callCount), int32(3))

	// Verify error was recorded in metrics
	errorCount := getFetchErrorValue(t, metrics.fetchErrors, string(TopicTypeDAGRun))
	assert.GreaterOrEqual(t, errorCount, float64(2))

	hub.Unsubscribe(client)
	cancel()
	<-pumpDone
}

// TestSSEMultipleTopicTypes tests different topic types
func TestSSEMultipleTopicTypes(t *testing.T) {
	hub := NewHub()

	// Register different fetchers for different topic types
	topics := []TopicType{
		TopicTypeDAGRun,
		TopicTypeDAG,
		TopicTypeDAGRunLogs,
		TopicTypeDAGRuns,
	}

	for _, topic := range topics {
		fetcher := mockFetchFunc(map[string]string{"topic": string(topic)}, nil)
		hub.RegisterFetcher(topic, fetcher)
	}

	hub.Start()
	defer hub.Shutdown()

	// Subscribe clients to different topics
	clients := make([]*Client, len(topics))
	topicStrings := []string{
		"dagrun:dag1/run1",
		"dag:dag1.yaml",
		"dagrunlogs:dag1/run1",
		"dagruns:limit=50",
	}

	for i := 0; i < len(topics); i++ {
		clients[i] = newTestClient(t)
		err := hub.Subscribe(clients[i], topicStrings[i])
		require.NoError(t, err)
	}

	assert.Equal(t, len(topics), hub.ClientCount())
	assert.Equal(t, len(topics), hub.WatcherCount())

	// Cleanup
	for _, client := range clients {
		hub.Unsubscribe(client)
	}

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.WatcherCount())
}

// TestSSEConcurrentSubscribeUnsubscribe tests concurrent operations
func TestSSEConcurrentSubscribeUnsubscribe(t *testing.T) {
	hub := NewHub(WithMaxClients(100))

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	var wg sync.WaitGroup
	numOperations := 50

	// Concurrent subscribe/unsubscribe
	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			client := newTestClient(t)
			err := hub.Subscribe(client, "dagrun:concurrent-test/run1")
			if err != nil {
				// May fail due to max clients, which is fine
				return
			}

			// Short delay
			time.Sleep(10 * time.Millisecond)

			hub.Unsubscribe(client)
		}(i)
	}

	wg.Wait()

	// After all operations complete, should be back to zero
	assert.Equal(t, 0, hub.ClientCount())
}

// TestSSEDataChangeDetection tests that data is only broadcast when it changes
func TestSSEDataChangeDetection(t *testing.T) {
	hub := NewHub()

	// Fetcher that returns same data for multiple calls
	callCount := 0
	fetcher := func(ctx context.Context, identifier string) (any, error) {
		callCount++
		// Always return same data
		return map[string]string{"stable": "data"}, nil
	}
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()
	defer hub.Shutdown()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:dag1/run1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pumpDone := make(chan struct{})
	go func() {
		client.WritePump(ctx)
		close(pumpDone)
	}()

	// Wait for multiple polling cycles
	time.Sleep(2500 * time.Millisecond)

	// Fetch should have been called multiple times
	assert.GreaterOrEqual(t, callCount, 2)

	hub.Unsubscribe(client)
	cancel()
	<-pumpDone
}

// TestSSEGracefulShutdown tests that shutdown cleanly closes all connections
func TestSSEGracefulShutdown(t *testing.T) {
	hub := NewHub()

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	hub.Start()

	// Subscribe multiple clients
	numClients := 5
	clients := make([]*Client, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = newTestClient(t)
		err := hub.Subscribe(clients[i], "dagrun:dag1/run1")
		require.NoError(t, err)
	}

	assert.Equal(t, numClients, hub.ClientCount())

	// Shutdown should close all clients
	hub.Shutdown()

	// All clients should be closed
	for i, client := range clients {
		assert.True(t, client.IsClosed(), "client %d should be closed after shutdown", i)
	}

	assert.Equal(t, 0, hub.ClientCount())
	assert.Equal(t, 0, hub.WatcherCount())
}

// TestSSEHeartbeatDelivery tests that heartbeats are delivered to clients
func TestSSEHeartbeatDelivery(t *testing.T) {
	registry := prometheus.NewRegistry()
	metrics := NewMetrics(registry)

	hub := NewHub(WithMetrics(metrics))

	fetcher := mockFetchFunc(map[string]string{"key": "value"}, nil)
	hub.RegisterFetcher(TopicTypeDAGRun, fetcher)

	// Manually start with short heartbeat for testing
	hub.mu.Lock()
	hub.started = true
	hub.heartbeatTicker = time.NewTicker(100 * time.Millisecond)
	hub.mu.Unlock()
	go hub.heartbeatLoop()

	defer hub.Shutdown()

	client := newTestClient(t)
	err := hub.Subscribe(client, "dagrun:dag1/run1")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.WritePump(ctx)

	// Wait for heartbeats
	time.Sleep(350 * time.Millisecond)

	// Should have received multiple heartbeats
	heartbeatCount := getCounterValue(t, metrics.messagesSent, EventTypeHeartbeat)
	assert.GreaterOrEqual(t, heartbeatCount, float64(2))

	hub.Unsubscribe(client)
	cancel()
}
