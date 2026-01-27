package agent

import (
	"context"
	"sync"
)

// SubPub provides a generic publish-subscribe mechanism for SSE streaming.
// It uses sequence-based subscriptions to ensure efficient delivery.
// Based on Shelley's subpub pattern.
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
	return &SubPub[K]{
		subscribers: make([]*subscriber[K], 0),
	}
}

// Subscribe registers interest in messages after the given sequence index.
// Returns a function that blocks until a new message is available.
// The second return value is false when the subscription is done.
func (sp *SubPub[K]) Subscribe(ctx context.Context, idx int64) func() (K, bool) {
	// Create a child context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	// Buffered channel to avoid blocking publishers
	ch := make(chan K, 10)
	sub := &subscriber[K]{
		idx:    idx,
		ch:     ch,
		ctx:    subCtx,
		cancel: cancel,
	}

	sp.mu.Lock()
	sp.subscribers = append(sp.subscribers, sub)
	sp.mu.Unlock()

	// Return a function that blocks until the next message
	return func() (K, bool) {
		select {
		case msg, ok := <-ch:
			if !ok {
				var zero K
				return zero, false
			}
			return msg, true
		case <-subCtx.Done():
			// Context cancelled, but drain any buffered messages first
			select {
			case msg, ok := <-ch:
				if ok {
					return msg, true
				}
			default:
			}
			var zero K
			return zero, false
		}
	}
}

// Publish sends a message to all subscribers waiting for messages after the given index.
// Subscribers that are "behind" (can't keep up) will be disconnected.
func (sp *SubPub[K]) Publish(idx int64, message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	// Notify subscribers and filter out disconnected ones
	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		// Check if context is still valid
		select {
		case <-sub.ctx.Done():
			// Context cancelled, close channel and don't keep subscriber
			close(sub.ch)
			continue
		default:
		}

		// Only send to subscribers waiting for messages after an index < idx
		if sub.idx < idx {
			// Try to send the message
			select {
			case sub.ch <- message:
				// Success, update subscriber's index and keep them
				sub.idx = idx
				remaining = append(remaining, sub)
			default:
				// Channel full, subscriber is behind - disconnect them
				close(sub.ch)
				sub.cancel()
			}
		} else {
			// This subscriber is not interested yet (already has this index or beyond)
			remaining = append(remaining, sub)
		}
	}
	sp.subscribers = remaining
}

// Broadcast sends a message to ALL subscribers regardless of their current index.
// This is used for out-of-band notifications like state changes.
func (sp *SubPub[K]) Broadcast(message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		select {
		case <-sub.ctx.Done():
			close(sub.ch)
			continue
		default:
		}

		select {
		case sub.ch <- message:
			remaining = append(remaining, sub)
		default:
			// Channel full, disconnect
			close(sub.ch)
			sub.cancel()
		}
	}
	sp.subscribers = remaining
}

// SubscriberCount returns the number of active subscribers.
func (sp *SubPub[K]) SubscriberCount() int {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	return len(sp.subscribers)
}
