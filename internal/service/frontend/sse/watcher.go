package sse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

// Watcher polls for data changes on a specific topic
// and broadcasts updates to all subscribed clients.
// It uses a pluggable FetchFunc to retrieve data, making it
// generic across different data sources.
type Watcher struct {
	topic      string          // Full topic string (e.g., "dagrun:mydag/run123")
	identifier string          // Identifier portion (e.g., "mydag/run123")
	fetcher    FetchFunc       // Function to fetch data for this topic
	clients    map[*Client]struct{}
	mu         sync.RWMutex
	lastHash   string
	stopCh     chan struct{}
	stopped    bool
}

// NewWatcher creates a new watcher for the given topic.
// The identifier is the portion after the topic type (e.g., "mydag/run123" for "dagrun:mydag/run123").
// The fetcher is called to retrieve data and should return the same structure as the REST API.
func NewWatcher(topic, identifier string, fetcher FetchFunc) *Watcher {
	return &Watcher{
		topic:      topic,
		identifier: identifier,
		fetcher:    fetcher,
		clients:    make(map[*Client]struct{}),
		stopCh:     make(chan struct{}),
	}
}

// Start begins polling for status changes
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	// Send initial state immediately
	w.poll(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.poll(ctx)
		}
	}
}

// Stop signals the watcher to stop polling
func (w *Watcher) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.stopped {
		w.stopped = true
		close(w.stopCh)
	}
}

// poll fetches the current data and broadcasts if changed.
func (w *Watcher) poll(ctx context.Context) {
	// Call the registered fetcher to get current data
	response, err := w.fetcher(ctx, w.identifier)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	// Marshal to JSON for comparison and transmission
	data, err := json.Marshal(response)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	// Only broadcast if data changed (hash-based comparison)
	if hash := computeHash(data); hash != w.lastHash {
		w.lastHash = hash
		w.broadcast(&Event{Type: EventTypeData, Data: string(data)})
	}
}

// broadcast sends an event to all subscribed clients
func (w *Watcher) broadcast(event *Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for client := range w.clients {
		if !client.Send(event) {
			// Client buffer full - close it asynchronously
			go client.Close()
		}
	}
}

// AddClient adds a client to this watcher's subscription list
func (w *Watcher) AddClient(client *Client) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.clients[client] = struct{}{}
}

// RemoveClient removes a client from this watcher's subscription list.
func (w *Watcher) RemoveClient(client *Client) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.clients, client)
}

// ClientCount returns the number of subscribed clients
func (w *Watcher) ClientCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.clients)
}

// computeHash computes a SHA256 hash of the data (first 16 hex chars)
func computeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:8])
}
