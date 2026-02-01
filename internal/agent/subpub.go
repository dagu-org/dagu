package agent

import (
	"context"
	"sync"
)

// defaultSubscriberBuffer is the buffer size for subscriber channels.
const defaultSubscriberBuffer = 10

// subscriber holds the state for a single subscription.
type subscriber[K any] struct {
	seqID     int64
	ch        chan K
	ctx       context.Context
	cancel    context.CancelFunc
	closeOnce sync.Once
}

// isDone checks if the subscriber's context is cancelled and closes the channel if so.
func (s *subscriber[K]) isDone() bool {
	select {
	case <-s.ctx.Done():
		s.closeOnce.Do(func() {
			close(s.ch)
		})
		return true
	default:
		return false
	}
}

// send attempts to send a message to the subscriber.
// Returns false and disconnects if the channel is full.
func (s *subscriber[K]) send(message K) bool {
	select {
	case s.ch <- message:
		return true
	default:
		s.closeOnce.Do(func() {
			close(s.ch)
			s.cancel()
		})
		return false
	}
}

// SubPub provides a generic publish-subscribe mechanism for SSE streaming.
// It uses sequence-based subscriptions to ensure efficient delivery.
type SubPub[K any] struct {
	mu          sync.Mutex
	subscribers []*subscriber[K]
}

// NewSubPub creates a new SubPub instance.
func NewSubPub[K any]() *SubPub[K] {
	return &SubPub[K]{}
}

// Subscribe registers interest in messages after the given sequence ID.
// Returns a function that blocks until the next message is available.
// The returned bool is false when the subscription ends.
func (sp *SubPub[K]) Subscribe(ctx context.Context, seqID int64) func() (K, bool) {
	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan K, defaultSubscriberBuffer) // Buffered to avoid blocking publishers

	sub := &subscriber[K]{
		seqID:  seqID,
		ch:     ch,
		ctx:    subCtx,
		cancel: cancel,
	}

	sp.mu.Lock()
	sp.subscribers = append(sp.subscribers, sub)
	sp.mu.Unlock()

	return makeReceiver(ch, subCtx)
}

// makeReceiver returns a function that receives messages from the channel.
func makeReceiver[K any](ch chan K, ctx context.Context) func() (K, bool) {
	var zero K
	return func() (K, bool) {
		select {
		case msg, ok := <-ch:
			if !ok {
				return zero, false
			}
			return msg, true

		case <-ctx.Done():
			// Drain one buffered message before returning if available
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

// Publish sends a message to all subscribers waiting for messages after the given sequence ID.
// Subscribers that cannot keep up will be disconnected.
func (sp *SubPub[K]) Publish(seqID int64, message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		if sub.isDone() {
			continue
		}

		if sub.seqID >= seqID {
			remaining = append(remaining, sub)
			continue
		}

		if sub.send(message) {
			sub.seqID = seqID
			remaining = append(remaining, sub)
		}
	}
	sp.subscribers = remaining
}

// Broadcast sends a message to all subscribers regardless of their current sequence ID.
// Used for out-of-band notifications like state changes.
func (sp *SubPub[K]) Broadcast(message K) {
	sp.mu.Lock()
	defer sp.mu.Unlock()

	remaining := sp.subscribers[:0]
	for _, sub := range sp.subscribers {
		if sub.isDone() {
			continue
		}

		if sub.send(message) {
			remaining = append(remaining, sub)
		}
	}
	sp.subscribers = remaining
}
