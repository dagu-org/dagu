package sse

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime"
	apiv2 "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
)

// Watcher polls for status changes on a specific DAG run topic
// and broadcasts updates to all subscribed clients
type Watcher struct {
	topic     string // e.g., "mydag/run123"
	dagName   string
	dagRunID  string
	dagRunMgr runtime.Manager
	clients   map[*Client]struct{}
	mu        sync.RWMutex
	lastHash  string
	stopCh    chan struct{}
	stopped   bool
}

// NewWatcher creates a new watcher for the given topic
func NewWatcher(topic string, dagRunMgr runtime.Manager) *Watcher {
	parts := strings.SplitN(topic, "/", 2)
	dagName := ""
	dagRunID := ""
	if len(parts) >= 1 {
		dagName = parts[0]
	}
	if len(parts) >= 2 {
		dagRunID = parts[1]
	}

	return &Watcher{
		topic:     topic,
		dagName:   dagName,
		dagRunID:  dagRunID,
		dagRunMgr: dagRunMgr,
		clients:   make(map[*Client]struct{}),
		stopCh:    make(chan struct{}),
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

// poll fetches the current status and broadcasts if changed
func (w *Watcher) poll(ctx context.Context) {
	ref := exec.NewDAGRunRef(w.dagName, w.dagRunID)

	// Direct call to runtime manager - NO HTTP LOGGING!
	status, err := w.dagRunMgr.GetSavedStatus(ctx, ref)
	if err != nil {
		// DAG run not found or error - notify clients
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	// Convert to API response format using exported transformer
	details := apiv2.ToDAGRunDetails(*status)
	data, err := json.Marshal(details)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	// Hash comparison
	currentHash := computeHash(data)
	if currentHash == w.lastHash {
		return // No change
	}
	w.lastHash = currentHash

	// Broadcast to all subscribers
	w.broadcast(&Event{Type: EventTypeData, Data: string(data)})
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

// RemoveClient removes a client from this watcher's subscription list
// Returns true if the client was found and removed
func (w *Watcher) RemoveClient(client *Client) bool {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.clients[client]; ok {
		delete(w.clients, client)
		return true
	}
	return false
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
