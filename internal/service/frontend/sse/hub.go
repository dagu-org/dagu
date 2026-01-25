package sse

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/runtime"
)

// HubOption is a functional option for configuring the Hub
type HubOption func(*Hub)

// WithMaxClients sets the maximum number of concurrent SSE clients
func WithMaxClients(max int) HubOption {
	return func(h *Hub) {
		h.maxClients = max
	}
}

// Hub manages all SSE client connections and their associated watchers
type Hub struct {
	mu              sync.RWMutex
	clients         map[*Client]string            // client -> topic
	watchers        map[string]*Watcher           // topic -> watcher
	dagRunMgr       runtime.Manager
	maxClients      int
	heartbeatTicker *time.Ticker
	ctx             context.Context
	cancel          context.CancelFunc
}

// NewHub creates a new SSE hub
func NewHub(dagRunMgr runtime.Manager, opts ...HubOption) *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:    make(map[*Client]string),
		watchers:   make(map[string]*Watcher),
		dagRunMgr:  dagRunMgr,
		maxClients: 1000, // Default limit
		ctx:        ctx,
		cancel:     cancel,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// Start begins the hub's background tasks (heartbeat, etc.)
func (h *Hub) Start() {
	h.heartbeatTicker = time.NewTicker(30 * time.Second)
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

// Subscribe adds a client to watch a specific topic
func (h *Hub) Subscribe(client *Client, topic string) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.clients) >= h.maxClients {
		return fmt.Errorf("max clients reached (%d)", h.maxClients)
	}

	h.clients[client] = topic

	// Get or create watcher for this topic
	watcher, ok := h.watchers[topic]
	if !ok {
		watcher = NewWatcher(topic, h.dagRunMgr)
		h.watchers[topic] = watcher
		go watcher.Start(h.ctx)
	}
	watcher.AddClient(client)

	return nil
}

// Unsubscribe removes a client from its topic
func (h *Hub) Unsubscribe(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	topic, ok := h.clients[client]
	if !ok {
		return
	}
	delete(h.clients, client)

	// Remove from watcher, stop if empty
	watcher, ok := h.watchers[topic]
	if !ok {
		return
	}

	watcher.RemoveClient(client)
	if watcher.ClientCount() == 0 {
		watcher.Stop()
		delete(h.watchers, topic)
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

// sendHeartbeats sends a heartbeat event to all connected clients
func (h *Hub) sendHeartbeats() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	heartbeat := &Event{Type: EventTypeHeartbeat, Data: "{}"}
	for client := range h.clients {
		if !client.Send(heartbeat) {
			// Client buffer full - close it asynchronously
			go func(c *Client) {
				c.Close()
				h.Unsubscribe(c)
			}(client)
		}
	}
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
