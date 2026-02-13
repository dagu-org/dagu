package agent

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubPub_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("receives messages via next function", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx := t.Context()

		next := sp.Subscribe(ctx, 0)

		go func() {
			time.Sleep(10 * time.Millisecond)
			sp.Publish(1, "hello")
		}()

		msg, ok := next()
		assert.True(t, ok)
		assert.Equal(t, "hello", msg)
	})

	t.Run("multiple subscribers receive same message", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx := t.Context()

		next1 := sp.Subscribe(ctx, 0)
		next2 := sp.Subscribe(ctx, 0)

		go func() {
			time.Sleep(10 * time.Millisecond)
			sp.Publish(1, "shared")
		}()

		msg1, ok1 := next1()
		msg2, ok2 := next2()

		assert.True(t, ok1)
		assert.True(t, ok2)
		assert.Equal(t, "shared", msg1)
		assert.Equal(t, "shared", msg2)
	})
}

func TestSubPub_Publish(t *testing.T) {
	t.Parallel()

	t.Run("filters by sequence ID", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := sp.Subscribe(ctx, 5)
		received := collectMessages(ctx, cancel, next)

		sp.Publish(3, "old")     // seqID 3 <= 5, should NOT be received
		sp.Publish(5, "current") // seqID 5 <= 5, should NOT be received
		sp.Publish(6, "new")     // seqID 6 > 5, SHOULD be received

		time.Sleep(50 * time.Millisecond)
		cancel()

		assert.Equal(t, []string{"new"}, <-received)
	})

	t.Run("updates subscriber seqID after receiving", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[int]()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := sp.Subscribe(ctx, 0)
		received := collectMessages(ctx, cancel, next)

		sp.Publish(1, 1)
		sp.Publish(2, 2)
		sp.Publish(3, 3)

		time.Sleep(50 * time.Millisecond)
		cancel()

		assert.Equal(t, []int{1, 2, 3}, <-received)
	})
}

func TestSubPub_Broadcast(t *testing.T) {
	t.Parallel()

	t.Run("sends to all subscribers regardless of seqID", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx := t.Context()

		next1 := sp.Subscribe(ctx, 0)
		next2 := sp.Subscribe(ctx, 100) // High seqID

		received1 := make(chan string, 1)
		received2 := make(chan string, 1)

		go func() {
			msg, ok := next1()
			if ok {
				received1 <- msg
			}
			close(received1)
		}()
		go func() {
			msg, ok := next2()
			if ok {
				received2 <- msg
			}
			close(received2)
		}()

		time.Sleep(10 * time.Millisecond)
		sp.Broadcast("announcement")
		time.Sleep(50 * time.Millisecond)

		assert.Equal(t, "announcement", <-received1)
		assert.Equal(t, "announcement", <-received2)
	})
}

func TestSubPub_ContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("next returns false when context is canceled", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithCancel(context.Background())

		next := sp.Subscribe(ctx, 0)

		done := make(chan bool)
		go func() {
			_, ok := next()
			done <- ok
		}()

		time.Sleep(10 * time.Millisecond)
		cancel()

		select {
		case ok := <-done:
			assert.False(t, ok)
		case <-time.After(time.Second):
			t.Fatal("next did not return after context cancellation")
		}
	})

	t.Run("subscriber removed after context canceled", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithCancel(context.Background())

		_ = sp.Subscribe(ctx, 0)
		cancel()

		time.Sleep(10 * time.Millisecond)

		// Publishing should clean up dead subscribers without panic
		sp.Publish(1, "test")
	})
}

func TestSubPub_SlowSubscriberDisconnect(t *testing.T) {
	t.Parallel()

	t.Run("disconnects subscriber when buffer is full", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[int]()
		ctx := t.Context()

		next := sp.Subscribe(ctx, 0)

		// Fill the buffer (size is 10) without reading
		for i := 1; i <= 15; i++ {
			sp.Publish(int64(i), i)
		}

		var received []int
		for {
			msg, ok := next()
			if !ok {
				break
			}
			received = append(received, msg)
			if len(received) > 20 {
				t.Fatal("should have disconnected by now")
			}
		}

		assert.NotEmpty(t, received)
	})
}

func TestSubPub_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	t.Run("handles concurrent publish and subscribe", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[int]()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		const numSubscribers = 10
		const numMessages = 100

		var wg sync.WaitGroup
		receivedCounts := make([]atomic.Int32, numSubscribers)

		for i := range numSubscribers {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				next := sp.Subscribe(ctx, 0)
				for {
					_, ok := next()
					if !ok {
						return
					}
					receivedCounts[idx].Add(1)
				}
			}(i)
		}

		time.Sleep(20 * time.Millisecond)

		var publishWg sync.WaitGroup
		for i := range numMessages {
			publishWg.Add(1)
			go func(seq int) {
				defer publishWg.Done()
				sp.Publish(int64(seq+1), seq)
			}(i)
		}
		publishWg.Wait()

		time.Sleep(100 * time.Millisecond)
		cancel()
		wg.Wait()

		for i := range receivedCounts {
			assert.Greater(t, receivedCounts[i].Load(), int32(0), "subscriber %d received no messages", i)
		}
	})

	t.Run("no race conditions with publish and broadcast", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		next := sp.Subscribe(ctx, 0)

		var wg sync.WaitGroup

		wg.Go(func() {
			for i := range 50 {
				sp.Publish(int64(i+1), "publish")
			}
		})

		wg.Go(func() {
			for range 50 {
				sp.Broadcast("broadcast")
			}
		})

		wg.Go(func() {
			for {
				_, ok := next()
				if !ok {
					return
				}
			}
		})

		wg.Wait()
	})
}

func TestMakeReceiver(t *testing.T) {
	t.Parallel()

	t.Run("drains buffered message on context cancel", func(t *testing.T) {
		t.Parallel()

		ch := make(chan string, 1)
		ctx, cancel := context.WithCancel(context.Background())

		receiver := makeReceiver(ch, ctx)

		ch <- "buffered"
		cancel()

		msg, ok := receiver()
		require.True(t, ok)
		assert.Equal(t, "buffered", msg)

		_, ok = receiver()
		assert.False(t, ok)
	})
}

// collectMessages starts a goroutine that collects all messages from the next function
// until the context is canceled. Returns a channel that receives the collected slice.
func collectMessages[K any](ctx context.Context, cancel context.CancelFunc, next func() (K, bool)) <-chan []K {
	result := make(chan []K, 1)
	go func() {
		var msgs []K
		for {
			msg, ok := next()
			if !ok {
				result <- msgs
				close(result)
				return
			}
			msgs = append(msgs, msg)
		}
	}()
	return result
}
