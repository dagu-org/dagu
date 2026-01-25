package sse

import (
	"context"
	"fmt"
	"net/http"
	"sync"
)

// Client represents a connected SSE client
type Client struct {
	w       http.ResponseWriter
	flusher http.Flusher
	send    chan *Event
	done    chan struct{}
	closed  bool
	mu      sync.Mutex
}

// NewClient creates a new SSE client from an HTTP response writer
func NewClient(w http.ResponseWriter) (*Client, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	return &Client{
		w:       w,
		flusher: flusher,
		send:    make(chan *Event, 64), // Buffered to handle bursts
		done:    make(chan struct{}),
	}, nil
}

// WritePump reads events from the send channel and writes them to the client.
// It blocks until the context is cancelled or the client is closed.
func (c *Client) WritePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case event, ok := <-c.send:
			if !ok {
				return
			}
			if err := c.writeEvent(event); err != nil {
				return
			}
		}
	}
}

// writeEvent writes a single SSE event to the client
func (c *Client) writeEvent(event *Event) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return fmt.Errorf("client closed")
	}

	// SSE format: "event: <type>\ndata: <json>\n\n"
	_, err := fmt.Fprintf(c.w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
	if err != nil {
		return err
	}
	c.flusher.Flush()
	return nil
}

// Send queues an event to be sent to the client.
// Returns false if the client buffer is full (slow client).
func (c *Client) Send(event *Event) bool {
	select {
	case c.send <- event:
		return true
	default:
		// Buffer full - client is too slow
		return false
	}
}

// Close closes the client connection
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.closed {
		c.closed = true
		close(c.done)
	}
}

// IsClosed returns true if the client has been closed
func (c *Client) IsClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}
