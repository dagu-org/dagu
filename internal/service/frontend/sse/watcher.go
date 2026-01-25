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

// Adaptive polling interval constants
const (
	defaultBaseInterval = time.Second      // Minimum polling interval
	defaultMaxInterval  = 10 * time.Second // Maximum polling interval cap
	intervalMultiplier  = 3                // interval = multiplier * fetchDuration
	smoothingFactor     = 0.3              // EMA alpha: weight for new value (0.3 = 30% new, 70% old)
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
	wg           sync.WaitGroup

	// Adaptive polling interval fields
	baseInterval    time.Duration
	maxInterval     time.Duration
	currentInterval time.Duration
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
		identifier:      identifier,
		topicType:       topicType,
		fetcher:         fetcher,
		clients:         make(map[*Client]struct{}),
		stopCh:          make(chan struct{}),
		errorBackoff:    backoff.NewRetrier(policy),
		metrics:         metrics,
		baseInterval:    defaultBaseInterval,
		maxInterval:     defaultMaxInterval,
		currentInterval: defaultBaseInterval,
	}
}

// Start begins polling for data changes and broadcasts updates to clients.
// Uses adaptive polling intervals based on fetch duration.
// IMPORTANT: Call wg.Add(1) before spawning the goroutine that calls this method.
// This method should be called via StartAsync for proper WaitGroup handling.
// Note: The Hub is responsible for tracking watcher metrics (WatcherStarted/WatcherStopped).
func (w *Watcher) Start(ctx context.Context) {
	// Use Timer instead of Ticker for variable intervals
	timer := time.NewTimer(0) // Immediate first poll
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-timer.C:
			w.poll(ctx)
			w.mu.RLock()
			interval := w.currentInterval
			w.mu.RUnlock()
			timer.Reset(interval) // Adaptive interval
		}
	}
}

// StartAsync starts the watcher in a background goroutine with proper WaitGroup handling.
// This ensures wg.Add(1) is called BEFORE the goroutine starts, preventing races with Stop().
func (w *Watcher) StartAsync(ctx context.Context) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.Start(ctx)
	}()
}

// Stop signals the watcher to stop polling and waits for it to finish.
// Note: The Hub is responsible for tracking watcher metrics (WatcherStarted/WatcherStopped).
func (w *Watcher) Stop() {
	w.mu.Lock()
	if !w.stopped {
		w.stopped = true
		close(w.stopCh)
	}
	w.mu.Unlock()
	w.wg.Wait()
}

// poll fetches the current data and broadcasts if changed.
// It measures fetch duration and adapts the polling interval accordingly.
func (w *Watcher) poll(ctx context.Context) {
	if w.isInBackoffPeriod() {
		return
	}

	start := time.Now()
	response, err := w.fetcher(ctx, w.identifier)
	fetchDuration := time.Since(start)

	if err != nil {
		w.handleFetchError(err)
		return
	}

	// Record fetch duration metric if available
	if w.metrics != nil {
		w.metrics.RecordFetchDuration(string(w.topicType), fetchDuration)
	}

	// Adapt polling interval based on fetch duration
	w.adaptInterval(fetchDuration)

	w.resetBackoff()
	w.broadcastIfChanged(response)
}

// adaptInterval adjusts the polling interval based on fetch duration.
// Uses EMA smoothing to prevent erratic interval changes.
func (w *Watcher) adaptInterval(fetchDuration time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Clamp target to bounds
	target := time.Duration(intervalMultiplier) * fetchDuration
	target = max(w.baseInterval, min(target, w.maxInterval))

	// EMA: new = alpha*target + (1-alpha)*current
	// Since both target and currentInterval are within bounds, the result is too
	w.currentInterval = time.Duration(
		float64(target)*smoothingFactor +
			float64(w.currentInterval)*(1-smoothingFactor),
	)
}

// isInBackoffPeriod returns true if we're still in an error backoff period.
func (w *Watcher) isInBackoffPeriod() bool {
	w.mu.RLock()
	until := w.backoffUntil
	w.mu.RUnlock()
	return time.Now().Before(until)
}

// handleFetchError applies exponential backoff and broadcasts the error.
func (w *Watcher) handleFetchError(err error) {
	w.mu.Lock()
	interval, _ := w.errorBackoff.Next(err)
	w.backoffUntil = time.Now().Add(interval)
	w.mu.Unlock()

	if w.metrics != nil {
		w.metrics.FetchError(string(w.topicType))
	}
	w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
}

// resetBackoff clears the backoff state after a successful fetch.
// If recovering from an error, also resets polling interval to base for responsiveness.
func (w *Watcher) resetBackoff() {
	w.mu.Lock()
	defer w.mu.Unlock()

	recoveringFromError := !w.backoffUntil.IsZero()

	w.errorBackoff.Reset()
	w.backoffUntil = time.Time{}

	if recoveringFromError {
		w.currentInterval = w.baseInterval
	}
}

// broadcastIfChanged marshals and broadcasts data only if it differs from last broadcast.
func (w *Watcher) broadcastIfChanged(response any) {
	data, err := json.Marshal(response)
	if err != nil {
		w.broadcast(&Event{Type: EventTypeError, Data: err.Error()})
		return
	}

	hash := computeHash(data)

	w.mu.Lock()
	changed := hash != w.lastHash
	if changed {
		w.lastHash = hash
	}
	w.mu.Unlock()

	if changed {
		w.broadcast(&Event{Type: EventTypeData, Data: string(data)})
	}
}

// broadcast sends an event to all subscribed clients.
func (w *Watcher) broadcast(event *Event) {
	// Collect clients under lock
	w.mu.RLock()
	clients := make([]*Client, 0, len(w.clients))
	for client := range w.clients {
		clients = append(clients, client)
	}
	w.mu.RUnlock()

	// Send outside lock to reduce contention
	for _, client := range clients {
		if client.Send(event) {
			w.metrics.MessageSent(event.Type)
		} else {
			client.Close()
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
