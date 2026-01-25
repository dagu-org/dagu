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
	identifier string
	fetcher    FetchFunc
	clients    map[*Client]struct{}
	mu         sync.RWMutex
	lastHash   string
	stopCh     chan struct{}
	stopped    bool
}

// NewWatcher creates a new watcher for the given topic.
// The identifier is the portion after the topic type (e.g., "mydag/run123" for "dagrun:mydag/run123").
// The fetcher is called to retrieve data and should return the same structure as the REST API.
func NewWatcher(identifier string, fetcher FetchFunc) *Watcher {
	return &Watcher{
		identifier: identifier,
		fetcher:    fetcher,
		clients:    make(map[*Client]struct{}),
		stopCh:     make(chan struct{}),
	}
}

// Start begins polling for data changes and broadcasts updates to clients.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	w.poll(ctx) // Initial state

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

// Stop signals the watcher to stop polling.
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
	response, err := w.fetcher(ctx, w.identifier)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	data, err := json.Marshal(response)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	hash := computeHash(data)
	if hash != w.lastHash {
		w.lastHash = hash
		w.broadcast(&Event{Type: EventTypeData, Data: string(data)})
	}
}

// broadcast sends an event to all subscribed clients.
func (w *Watcher) broadcast(event *Event) {
	w.mu.RLock()
	defer w.mu.RUnlock()

	for client := range w.clients {
		if !client.Send(event) {
			go client.Close() // Buffer full
		}
	}
}

// AddClient adds a client to this watcher's subscription list.
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

// ClientCount returns the number of subscribed clients.
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
