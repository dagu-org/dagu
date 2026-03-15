// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeSessionContext_CancelsWhenEitherContextEnds(t *testing.T) {
	t.Parallel()

	t.Run("RequestContext", func(t *testing.T) {
		requestCtx, cancelRequest := context.WithCancel(context.Background())
		managerCtx := t.Context()

		ctx, cancel := mergeSessionContext(requestCtx, managerCtx)
		defer cancel()

		cancelRequest()
		waitForContextDone(t, ctx)
	})

	t.Run("ManagerContext", func(t *testing.T) {
		requestCtx := t.Context()
		managerCtx, cancelManager := context.WithCancel(context.Background())

		ctx, cancel := mergeSessionContext(requestCtx, managerCtx)
		defer cancel()

		cancelManager()
		waitForContextDone(t, ctx)
	})
}

func TestConnection_ClassifyWebSocketError(t *testing.T) {
	t.Parallel()

	t.Run("Shutdown", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		event := classifyWebSocketEvent(ctx, context.Canceled, websocketOpRead)
		assert.Equal(t, terminalEndReasonShutdown, event.reason)
		assert.NoError(t, event.err)
	})

	t.Run("ClientClose", func(t *testing.T) {
		event := classifyWebSocketEvent(context.Background(), websocket.CloseError{
			Code: websocket.StatusNormalClosure,
		}, websocketOpRead)
		assert.Equal(t, terminalEndReasonClientClose, event.reason)
		assert.NoError(t, event.err)
	})

	t.Run("Disconnect", func(t *testing.T) {
		event := classifyWebSocketEvent(context.Background(), errors.New("network drop"), websocketOpRead)
		assert.Equal(t, terminalEndReasonDisconnect, event.reason)
		assert.NoError(t, event.err)
	})
}

func TestConnection_ClassifyWebSocketWriteError(t *testing.T) {
	t.Parallel()

	t.Run("ExpectedDisconnect", func(t *testing.T) {
		event := classifyWebSocketEvent(context.Background(), net.ErrClosed, websocketOpWrite)
		assert.Equal(t, terminalEndReasonDisconnect, event.reason)
		assert.NoError(t, event.err)
	})

	t.Run("CloseFrame", func(t *testing.T) {
		event := classifyWebSocketEvent(context.Background(), websocket.CloseError{
			Code: websocket.StatusAbnormalClosure,
		}, websocketOpWrite)
		assert.Equal(t, terminalEndReasonDisconnect, event.reason)
		assert.NoError(t, event.err)
	})

	t.Run("UnexpectedWriteError", func(t *testing.T) {
		errBoom := errors.New("boom")
		event := classifyWebSocketEvent(context.Background(), errBoom, websocketOpWrite)
		assert.Equal(t, terminalEndReasonDisconnect, event.reason)
		require.Error(t, event.err)
		assert.ErrorIs(t, event.err, errBoom)
	})

	t.Run("EOF", func(t *testing.T) {
		event := classifyWebSocketEvent(context.Background(), io.EOF, websocketOpWrite)
		assert.Equal(t, terminalEndReasonDisconnect, event.reason)
		assert.NoError(t, event.err)
	})
}

func TestConnection_ShouldSuppressPTYReadErrorOnCanceledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	conn := &Connection{}
	assert.True(t, conn.shouldSuppressPTYReadError(ctx, os.ErrDeadlineExceeded, make(chan struct{})))
}

func TestConnection_InterruptTransportUsesExpectedWebSocketCloseMode(t *testing.T) {
	t.Parallel()

	t.Run("Graceful", func(t *testing.T) {
		fake := &fakeWebSocketConn{}
		conn := &Connection{Conn: fake}

		conn.interruptTransport(true)

		assert.Equal(t, 1, fake.closeCalls)
		assert.Equal(t, 0, fake.closeNowCalls)
		assert.Equal(t, websocket.StatusNormalClosure, fake.closeStatus)
		assert.Equal(t, "connection closed", fake.closeReason)
	})

	t.Run("Forced", func(t *testing.T) {
		fake := &fakeWebSocketConn{}
		conn := &Connection{Conn: fake}

		conn.interruptTransport(false)

		assert.Equal(t, 0, fake.closeCalls)
		assert.Equal(t, 1, fake.closeNowCalls)
	})
}

func TestConnection_ForceKillDoesNotBypassCleanup(t *testing.T) {
	t.Parallel()

	fake := &fakeWebSocketConn{}
	conn := &Connection{Conn: fake}

	var ioWG sync.WaitGroup
	processDone := make(chan struct{})
	close(processDone)

	_, cancel := context.WithCancel(context.Background())
	conn.ForceKill()
	require.NoError(t, conn.cleanup(cancel, &ioWG, processDone, false))

	assert.Equal(t, 2, fake.closeNowCalls)
	assert.True(t, conn.isClosing())
}

func waitForContextDone(t *testing.T, ctx context.Context) {
	t.Helper()

	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not cancelled")
	}
}

type fakeWebSocketConn struct {
	closeCalls    int
	closeNowCalls int
	closeStatus   websocket.StatusCode
	closeReason   string
}

func (f *fakeWebSocketConn) Read(context.Context) (websocket.MessageType, []byte, error) {
	return websocket.MessageText, nil, errors.New("unexpected read")
}

func (f *fakeWebSocketConn) Write(context.Context, websocket.MessageType, []byte) error {
	return nil
}

func (f *fakeWebSocketConn) Close(status websocket.StatusCode, reason string) error {
	f.closeCalls++
	f.closeStatus = status
	f.closeReason = reason
	return nil
}

func (f *fakeWebSocketConn) CloseNow() error {
	f.closeNowCalls++
	return nil
}
