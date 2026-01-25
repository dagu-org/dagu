package sse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
)

// Watcher polls for data changes on a specific topic
// and broadcasts updates to all subscribed clients.
// It uses a pluggable FetchFunc to retrieve data, making it
// generic across different data sources.
type Watcher struct {
	identifier   string
	topicType    TopicType
	fetcher      FetchFunc
	clients      map[*Client]struct{}
	mu           sync.RWMutex
	lastHash     string
	stopCh       chan struct{}
	stopped      bool
	errorBackoff backoff.Retrier
	backoffUntil time.Time
	metrics      *Metrics
}

// NewWatcher creates a new watcher for the given topic.
// The identifier is the portion after the topic type (e.g., "mydag/run123" for "dagrun:mydag/run123").
// The fetcher is called to retrieve data and should return the same structure as the REST API.
// The topicType is used for metrics labeling. Pass an empty TopicType if metrics are not needed.
// The metrics parameter can be nil if metrics collection is not required.
func NewWatcher(identifier string, fetcher FetchFunc, topicType TopicType, metrics *Metrics) *Watcher {
	// Create exponential backoff policy: 1s initial, 2x factor, 30s max
	policy := backoff.NewExponentialBackoffPolicy(time.Second)
	policy.MaxInterval = 30 * time.Second

	return &Watcher{
		identifier:   identifier,
		topicType:    topicType,
		fetcher:      fetcher,
		clients:      make(map[*Client]struct{}),
		stopCh:       make(chan struct{}),
		errorBackoff: backoff.NewRetrier(policy),
		metrics:      metrics,
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
	// Skip polling during backoff period
	if time.Now().Before(w.backoffUntil) {
		return
	}

	response, err := w.fetcher(ctx, w.identifier)
	if err != nil {
		// Calculate next backoff interval
		interval, _ := w.errorBackoff.Next(err)
		w.backoffUntil = time.Now().Add(interval)
		if w.metrics != nil {
			w.metrics.FetchError(string(w.topicType))
		}
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	// Reset backoff on successful fetch
	w.errorBackoff.Reset()
	w.backoffUntil = time.Time{}

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
		} else if w.metrics != nil {
			w.metrics.MessageSent(event.Type)
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
