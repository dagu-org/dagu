package sse

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxClients = 1000
	heartbeatInterval = 10 * time.Second
)

// HubOption is a functional option for configuring the Hub.
type HubOption func(*Hub)

// WithMaxClients sets the maximum number of concurrent SSE clients.
func WithMaxClients(max int) HubOption {
	return func(h *Hub) {
		h.maxClients = max
	}
}

// WithMetrics sets the metrics instance for the hub.
func WithMetrics(m *Metrics) HubOption {
	return func(h *Hub) {
		h.metrics = m
	}
}

// Hub manages all SSE client connections and their associated watchers.
// It uses a pluggable fetcher pattern where each TopicType has a registered
// FetchFunc that knows how to retrieve data for that topic type.
type Hub struct {
	mu              sync.RWMutex
	clients         map[*Client]string      // client -> topic
	watchers        map[string]*Watcher     // topic -> watcher
	fetchers        map[TopicType]FetchFunc // topicType -> fetcher
	maxClients      int
	heartbeatTicker *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
	metrics         *Metrics
	started         bool
}

// NewHub creates a new SSE hub.
// Use RegisterFetcher to add data fetchers for each topic type before subscribing clients.
func NewHub(opts ...HubOption) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:    make(map[*Client]string),
		watchers:   make(map[string]*Watcher),
		fetchers:   make(map[TopicType]FetchFunc),
		maxClients: defaultMaxClients,
		ctx:        ctx,
		cancel:     cancel,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RegisterFetcher registers a data fetcher for a specific topic type.
// The fetcher will be called by watchers to retrieve data for topics of this type.
// This must be called before any clients subscribe to topics of this type.
func (h *Hub) RegisterFetcher(topicType TopicType, fetcher FetchFunc) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.fetchers[topicType] = fetcher
}

// Start begins the hub's background tasks (heartbeat, etc.)
// Safe to call multiple times; only the first call has effect.
func (h *Hub) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return
	}
	h.started = true
	h.heartbeatTicker = time.NewTicker(heartbeatInterval)
	go h.heartbeatLoop()
}

// Shutdown gracefully shuts down the hub, closing all clients and watchers
func (h *Hub) Shutdown() {
	h.cancel()
	if h.heartbeatTicker != nil {
		h.heartbeatTicker.Stop()
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Close all clients
	for client := range h.clients {
		client.Close()
	}

	// Stop all watchers
	for _, watcher := range h.watchers {
		watcher.Stop()
	}

	// Clear maps
	h.clients = make(map[*Client]string)
	h.watchers = make(map[string]*Watcher)
}

// Subscribe adds a client to watch a specific topic.
// Topic format: "topicType:identifier" (e.g., "dagrun:mydag/run123")
func (h *Hub) Subscribe(client *Client, topic string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients) >= h.maxClients {
		return fmt.Errorf("max clients reached (%d)", h.maxClients)
	}

	// Parse topic format: "type:identifier"
	topicType, identifier, ok := strings.Cut(topic, ":")
	if !ok {
		return fmt.Errorf("invalid topic format: %s (expected 'type:identifier')", topic)
	}

	// Look up the fetcher for this topic type
	fetcher, ok := h.fetchers[TopicType(topicType)]
	if !ok {
		return fmt.Errorf("no fetcher registered for topic type: %s", topicType)
	}

	h.clients[client] = topic
	if h.metrics != nil {
		h.metrics.ClientConnected()
	}

	watcher, ok := h.watchers[topic]
	if !ok {
		watcher = NewWatcher(identifier, fetcher, TopicType(topicType), h.metrics)
		h.watchers[topic] = watcher
		if h.metrics != nil {
			h.metrics.WatcherStarted()
		}
		go watcher.Start(h.ctx)
	}
	watcher.AddClient(client)

	return nil
}

// Unsubscribe removes a client from its topic.
func (h *Hub) Unsubscribe(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	topic, ok := h.clients[client]
	if !ok {
		return
	}
	delete(h.clients, client)
	if h.metrics != nil {
		h.metrics.ClientDisconnected()
	}

	watcher, ok := h.watchers[topic]
	if !ok {
		return
	}

	watcher.RemoveClient(client)
	if watcher.ClientCount() == 0 {
		watcher.Stop()
		delete(h.watchers, topic)
		if h.metrics != nil {
			h.metrics.WatcherStopped()
		}
	}
}

// heartbeatLoop sends heartbeat events to all clients periodically
func (h *Hub) heartbeatLoop() {
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-h.heartbeatTicker.C:
			h.sendHeartbeats()
		}
	}
}

// sendHeartbeats sends a heartbeat event to all connected clients.
func (h *Hub) sendHeartbeats() {
	clients := h.collectClients()
	heartbeat := &Event{Type: EventTypeHeartbeat, Data: "{}"}

	for _, client := range clients {
		if !client.Send(heartbeat) {
			client.Close()
			h.Unsubscribe(client)
			continue
		}
		h.recordMessageSent(EventTypeHeartbeat)
	}
}

// recordMessageSent records a sent message metric if metrics are enabled.
func (h *Hub) recordMessageSent(eventType string) {
	if h.metrics != nil {
		h.metrics.MessageSent(eventType)
	}
}

// collectClients returns a snapshot of all connected clients.
func (h *Hub) collectClients() []*Client {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients := make([]*Client, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, client)
	}
	return clients
}

// ClientCount returns the current number of connected clients
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// WatcherCount returns the current number of active watchers
func (h *Hub) WatcherCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.watchers)
}
