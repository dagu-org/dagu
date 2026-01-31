package agent

import (
	"context"
	"sync"
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := sp.Subscribe(ctx, 0)

		// Publish a message
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
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

		// Subscribe with seqID 5 - should only receive messages with seqID > 5
		next := sp.Subscribe(ctx, 5)

		received := make(chan string, 10)
		go func() {
			for {
				msg, ok := next()
				if !ok {
					close(received)
					return
				}
				received <- msg
			}
		}()

		// This should NOT be received (seqID 3 <= 5)
		sp.Publish(3, "old")
		// This should NOT be received (seqID 5 <= 5)
		sp.Publish(5, "current")
		// This SHOULD be received (seqID 6 > 5)
		sp.Publish(6, "new")

		// Give time for processing
		time.Sleep(50 * time.Millisecond)
		cancel()

		// Collect received messages
		var msgs []string
		for msg := range received {
			msgs = append(msgs, msg)
		}

		assert.Equal(t, []string{"new"}, msgs)
	})

	t.Run("updates subscriber seqID after receiving", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[int]()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := sp.Subscribe(ctx, 0)

		received := make(chan int, 10)
		go func() {
			for {
				msg, ok := next()
				if !ok {
					close(received)
					return
				}
				received <- msg
			}
		}()

		// Publish sequentially
		sp.Publish(1, 1)
		sp.Publish(2, 2)
		sp.Publish(3, 3)

		time.Sleep(50 * time.Millisecond)
		cancel()

		var msgs []int
		for msg := range received {
			msgs = append(msgs, msg)
		}

		assert.Equal(t, []int{1, 2, 3}, msgs)
	})
}

func TestSubPub_Broadcast(t *testing.T) {
	t.Parallel()

	t.Run("sends to all subscribers regardless of seqID", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Subscribe with different seqIDs
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

		msg1 := <-received1
		msg2 := <-received2

		assert.Equal(t, "announcement", msg1)
		assert.Equal(t, "announcement", msg2)
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

		// Cancel context
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

		// Give time for subscriber to be cleaned up
		time.Sleep(10 * time.Millisecond)

		// Publishing should clean up dead subscribers
		sp.Publish(1, "test")

		// Verify no panic or issues
	})
}

func TestSubPub_SlowSubscriberDisconnect(t *testing.T) {
	t.Parallel()

	t.Run("disconnects subscriber when buffer is full", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[int]()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		next := sp.Subscribe(ctx, 0)

		// Fill the buffer (size is 10) without reading
		for i := 1; i <= 15; i++ {
			sp.Publish(int64(i), i)
		}

		// Now try to read - should get some messages then false
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

		// Should have received some messages before disconnect
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

		// Start subscribers
		receivedCounts := make([]int, numSubscribers)
		for i := 0; i < numSubscribers; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				next := sp.Subscribe(ctx, 0)
				for {
					_, ok := next()
					if !ok {
						return
					}
					receivedCounts[idx]++
				}
			}(i)
		}

		// Give subscribers time to register
		time.Sleep(20 * time.Millisecond)

		// Publish messages concurrently
		var publishWg sync.WaitGroup
		for i := 0; i < numMessages; i++ {
			publishWg.Add(1)
			go func(seq int) {
				defer publishWg.Done()
				sp.Publish(int64(seq+1), seq)
			}(i)
		}
		publishWg.Wait()

		// Give time for delivery
		time.Sleep(100 * time.Millisecond)
		cancel()
		wg.Wait()

		// Each subscriber should have received some messages
		for i, count := range receivedCounts {
			assert.Greater(t, count, 0, "subscriber %d received no messages", i)
		}
	})

	t.Run("no race conditions with publish and broadcast", func(t *testing.T) {
		t.Parallel()

		sp := NewSubPub[string]()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		next := sp.Subscribe(ctx, 0)

		var wg sync.WaitGroup

		// Concurrent publishes
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				sp.Publish(int64(i+1), "publish")
			}
		}()

		// Concurrent broadcasts
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				sp.Broadcast("broadcast")
			}
		}()

		// Concurrent reads
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				_, ok := next()
				if !ok {
					return
				}
			}
		}()

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

		// Buffer a message before canceling
		ch <- "buffered"
		cancel()

		// Should still receive the buffered message
		msg, ok := receiver()
		require.True(t, ok)
		assert.Equal(t, "buffered", msg)

		// Next call should return false
		_, ok = receiver()
		assert.False(t, ok)
	})
}
