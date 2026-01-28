package agent

import (
	"context"
	"sync"
)

// SubPub provides a generic publish-subscribe mechanism for SSE streaming.
// It uses sequence-based subscriptions to ensure efficient delivery.
type SubPub[K any] struct {
	mu          sync.Mutex
	subscribers []*subscriber[K]
}

type subscriber[K any] struct {
	idx    int64
	ch     chan K
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSubPub creates a new SubPub instance.
func NewSubPub[K any]() *SubPub[K] {
	return &SubPub[K]{}
}

// Subscribe registers interest in messages after the given sequence index.
// Returns a function that blocks until the next message is available.
// The returned bool is false when the subscription ends.
func (sp *SubPub[K]) Subscribe(ctx context.Context, idx int64) func() (K, bool) {
	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan K, 10) // Buffered to avoid blocking publishers

	sub := &subscriber[K]{
		idx:    idx,
		ch:     ch,
		ctx:    subCtx,
		cancel: cancel,
	}

	sp.mu.Lock()
	sp.subscribers = append(sp.subscribers, sub)
	sp.mu.Unlock()

	var zero K
	return func() (K, bool) {
		select {
		case msg, ok := <-ch:
			if !ok {
				return zero, false
			}
			return msg, true

		case <-subCtx.Done():
			// Try to drain one buffered message before returning
			select {
			case msg, ok := <-ch:
				if ok {
					return msg, true
				}
			default:
			}
			return zero, false
		}
	}
}

// Publish sends a message to all subscribers waiting for messages after the given index.
// Subscribers that cannot keep up will be disconnected.
func (sp *SubPub[K]) Publish(idx int64, message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		if sp.isContextDone(sub) {
			continue
		}

		if sub.idx >= idx {
			// Subscriber already has this index or beyond
			remaining = append(remaining, sub)
			continue
		}

		if sp.trySend(sub, message) {
			sub.idx = idx
			remaining = append(remaining, sub)
		}
	}
	sp.subscribers = remaining
}

// Broadcast sends a message to all subscribers regardless of their current index.
// Used for out-of-band notifications like state changes.
func (sp *SubPub[K]) Broadcast(message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		if sp.isContextDone(sub) {
			continue
		}

		if sp.trySend(sub, message) {
			remaining = append(remaining, sub)
		}
	}
	sp.subscribers = remaining
}

// isContextDone checks if a subscriber's context is cancelled and cleans up if so.
// Must be called with sp.mu held.
func (*SubPub[K]) isContextDone(sub *subscriber[K]) bool {
	select {
	case <-sub.ctx.Done():
		close(sub.ch)
		return true
	default:
		return false
	}
}

// trySend attempts to send a message to a subscriber.
// Returns false and disconnects the subscriber if the channel is full.
// Must be called with sp.mu held.
func (*SubPub[K]) trySend(sub *subscriber[K], message K) bool {
	select {
	case sub.ch <- message:
		return true
	default:
		close(sub.ch)
		sub.cancel()
		return false
	}
}
