package sse

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFlusher implements http.ResponseWriter with Flusher interface
type mockFlusher struct {
	*httptest.ResponseRecorder
	flushCount int
}

func (m *mockFlusher) Flush() {
	m.flushCount++
}

// nonFlusher implements http.ResponseWriter without Flusher interface
type nonFlusher struct {
	bytes.Buffer
	Code int
}

func (n *nonFlusher) Header() http.Header        { return http.Header{} }
func (n *nonFlusher) WriteHeader(statusCode int) { n.Code = statusCode }
func (n *nonFlusher) Write(b []byte) (int, error) {
	// Set default 200 if no explicit WriteHeader was called
	if n.Code == 0 {
		n.Code = http.StatusOK
	}
	return n.Buffer.Write(b)
}

func newMockFlusher() *mockFlusher {
	return &mockFlusher{
		ResponseRecorder: httptest.NewRecorder(),
	}
}

func TestNewClient(t *testing.T) {
	t.Parallel()
	t.Run("success with flusher", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()

		client, err := NewClient(w)

		require.NoError(t, err)
		require.NotNil(t, client)
		assert.NotNil(t, client.send)
		assert.NotNil(t, client.done)
		assert.False(t, client.closed)
	})

	t.Run("error without flusher", func(t *testing.T) {
		t.Parallel()
		w := &nonFlusher{}

		client, err := NewClient(w)

		require.Error(t, err)
		assert.Equal(t, ErrStreamingNotSupported, err)
		assert.Nil(t, client)
	})
}

func TestClientSend(t *testing.T) {
	t.Parallel()
	t.Run("normal send", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		event := &Event{Type: EventTypeData, Data: "test"}
		ok := client.Send(event)

		assert.True(t, ok)

		// Verify event is in channel
		received := <-client.send
		assert.Equal(t, event, received)
	})

	t.Run("buffer overflow", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		// Fill the buffer (64 items)
		for i := 0; i < 64; i++ {
			ok := client.Send(&Event{Type: EventTypeData, Data: "test"})
			assert.True(t, ok, "send %d should succeed", i)
		}

		// Next send should fail (buffer full)
		ok := client.Send(&Event{Type: EventTypeData, Data: "overflow"})
		assert.False(t, ok, "send should fail when buffer is full")
	})
}

func TestClientWritePump(t *testing.T) {
	t.Parallel()
	t.Run("writes events", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start write pump in background
		done := make(chan struct{})
		go func() {
			client.WritePump(ctx)
			close(done)
		}()

		// Send an event
		event := &Event{Type: EventTypeData, Data: `{"key":"value"}`}
		ok := client.Send(event)
		require.True(t, ok)

		// Give time for the event to be processed
		time.Sleep(50 * time.Millisecond)

		// Verify the output
		output := w.Body.String()
		assert.Contains(t, output, "event: data")
		assert.Contains(t, output, `data: {"key":"value"}`)

		// Verify flush was called
		assert.GreaterOrEqual(t, w.flushCount, 1)

		// Cancel context to stop pump
		cancel()

		// Wait for pump to stop
		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Fatal("WritePump did not stop on context cancel")
		}
	})

	t.Run("stops on context cancel", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		ctx, cancel := context.WithCancel(context.Background())

		done := make(chan struct{})
		go func() {
			client.WritePump(ctx)
			close(done)
		}()

		// Cancel immediately
		cancel()

		select {
		case <-done:
			// Success - pump stopped
		case <-time.After(time.Second):
			t.Fatal("WritePump did not stop on context cancel")
		}
	})

	t.Run("stops on client close", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		ctx := context.Background()

		done := make(chan struct{})
		go func() {
			client.WritePump(ctx)
			close(done)
		}()

		// Close the client
		client.Close()

		select {
		case <-done:
			// Success - pump stopped
		case <-time.After(time.Second):
			t.Fatal("WritePump did not stop on client close")
		}
	})

	t.Run("stops on nil event", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		ctx := context.Background()

		done := make(chan struct{})
		go func() {
			client.WritePump(ctx)
			close(done)
		}()

		// Send nil event
		client.send <- nil

		select {
		case <-done:
			// Success - pump stopped
		case <-time.After(time.Second):
			t.Fatal("WritePump did not stop on nil event")
		}
	})
}

func TestClientClose(t *testing.T) {
	t.Parallel()
	t.Run("closes client", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		assert.False(t, client.IsClosed())

		client.Close()

		assert.True(t, client.IsClosed())
	})

	t.Run("idempotent close", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		// Close multiple times - should not panic
		assert.NotPanics(t, func() {
			client.Close()
			client.Close()
			client.Close()
		})

		assert.True(t, client.IsClosed())
	})

	t.Run("done channel is closed", func(t *testing.T) {
		t.Parallel()
		w := newMockFlusher()
		client, err := NewClient(w)
		require.NoError(t, err)

		client.Close()

		// done channel should be closed
		select {
		case <-client.done:
			// Success - channel is closed
		default:
			t.Fatal("done channel should be closed")
		}
	})
}

func TestClientConcurrentSend(t *testing.T) {
	t.Parallel()
	w := newMockFlusher()
	client, err := NewClient(w)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start write pump
	go client.WritePump(ctx)

	// Concurrent sends
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				client.Send(&Event{Type: EventTypeData, Data: "concurrent"})
			}
		}()
	}

	wg.Wait()

	// Give time for events to be processed
	time.Sleep(100 * time.Millisecond)

	// Verify some events were written (some may have been dropped due to buffer)
	output := w.Body.String()
	assert.Contains(t, output, "event: data")
}

func TestClientWriteEventWhenClosed(t *testing.T) {
	t.Parallel()
	w := newMockFlusher()
	client, err := NewClient(w)
	require.NoError(t, err)

	client.Close()

	err = client.writeEvent(&Event{Type: EventTypeData, Data: "test"})
	assert.Equal(t, ErrClientClosed, err)
}

// failingWriter is a mock writer that fails on Write
type failingWriter struct{}

func (f *failingWriter) Header() http.Header { return http.Header{} }
func (f *failingWriter) WriteHeader(_ int)   {}
func (f *failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}
func (f *failingWriter) Flush() {}

func TestClientWriteEventError(t *testing.T) {
	t.Parallel()
	w := &failingWriter{}
	client, err := NewClient(w)
	require.NoError(t, err)

	// This should return an error because the writer fails
	err = client.writeEvent(&Event{Type: EventTypeData, Data: "test"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write failed")
}

func TestClientWritePumpWriteError(t *testing.T) {
	t.Parallel()
	w := &failingWriter{}
	client, err := NewClient(w)
	require.NoError(t, err)

	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		client.WritePump(ctx)
		close(done)
	}()

	// Send an event - the write should fail and pump should return
	client.Send(&Event{Type: EventTypeData, Data: "test"})

	select {
	case <-done:
		// Success - pump stopped due to write error
	case <-time.After(time.Second):
		t.Fatal("WritePump did not stop on write error")
	}
}
