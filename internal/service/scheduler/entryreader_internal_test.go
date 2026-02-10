package scheduler

import (
	"context"
	"testing"
	"time"
)

func TestSendEvent_UnblocksOnQuit(t *testing.T) {
	t.Parallel()

	er := &entryReaderImpl{
		events: make(chan DAGChangeEvent), // unbuffered
		quit:   make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		er.sendEvent(context.Background(), DAGChangeEvent{
			Type:    DAGChangeAdded,
			DAGName: "test",
		})
		close(done)
	}()

	// Give sendEvent time to block
	time.Sleep(50 * time.Millisecond)

	// Close quit — this should unblock sendEvent
	close(er.quit)

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("sendEvent did not unblock after quit was closed")
	}
}

func TestSendEvent_UnblocksOnContextCancel(t *testing.T) {
	t.Parallel()

	er := &entryReaderImpl{
		events: make(chan DAGChangeEvent), // unbuffered
		quit:   make(chan struct{}),
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		er.sendEvent(ctx, DAGChangeEvent{
			Type:    DAGChangeAdded,
			DAGName: "test",
		})
		close(done)
	}()

	// Give sendEvent time to block
	time.Sleep(50 * time.Millisecond)

	// Cancel context — this should unblock sendEvent
	cancel()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("sendEvent did not unblock after context cancel")
	}
}

func TestSendEvent_NilChannelReturnsImmediately(t *testing.T) {
	t.Parallel()

	er := &entryReaderImpl{
		events: nil,
		quit:   make(chan struct{}),
	}

	done := make(chan struct{})
	go func() {
		er.sendEvent(context.Background(), DAGChangeEvent{
			Type:    DAGChangeAdded,
			DAGName: "test",
		})
		close(done)
	}()

	select {
	case <-done:
		// success
	case <-time.After(2 * time.Second):
		t.Fatal("sendEvent blocked on nil channel")
	}
}
