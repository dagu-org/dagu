// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	acp "github.com/coder/acp-go-sdk"
	"github.com/coder/websocket"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime"
)

var acpWebSocketHTTPClient = &http.Client{
	Transport: newOpenCodeTransport(),
	// Remote endpoints must not forward credentials through redirects.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// runACPWebSocket executes a turn over an ACP UI-compatible remote connection.
// The working directory is passed to the agent unchanged, never created locally.
func (e *harnessExecutor) runACPWebSocket(parent context.Context, cfg providerConfig, out io.Writer, allow bool) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	e.mu.Lock()
	e.acpCancel = cancel
	e.mu.Unlock()
	defer func() {
		e.mu.Lock()
		e.acpCancel = nil
		e.mu.Unlock()
	}()
	for key := range cfg.flags {
		if !reservedKeys[key] && key != "acp_permission" {
			return fmt.Errorf("harness: remote ACP does not accept CLI option %q", key)
		}
	}
	settings, err := runtime.EvalObject(ctx, struct {
		URL     string
		Cwd     string
		Headers map[string]string
	}{cfg.definition.URL, cfg.definition.Cwd, cfg.definition.Headers})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("harness: could not resolve remote ACP connection settings")
	}
	def := cloneDefinition(cfg.definition)
	def.URL, def.Cwd, def.Headers = settings.URL, settings.Cwd, settings.Headers
	if err := def.ValidateConnection(); err != nil {
		return fmt.Errorf("harness: remote ACP: %w", err)
	}
	if strings.Contains(def.URL, "${") || strings.Contains(def.Cwd, "${") {
		return errors.New("harness: unresolved remote ACP connection settings")
	}
	socket, err := dialACPWebSocket(ctx, def, acpWebSocketHTTPClient)
	if err != nil {
		return err
	}
	// Keep the transport alive until session/cancel has had a chance to be sent.
	wireCtx, closeWire := context.WithCancel(context.Background())
	defer closeWire()
	stream := &acpWebSocketStream{ctx: wireCtx, conn: socket}
	client := &acpHarnessClient{out: out, allow: allow}
	conn := acp.NewClientSideConnection(client, stream, stream)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		keepACPWebSocketAlive(wireCtx, socket)
	}()
	defer func() {
		closeWire()
		_ = socket.CloseNow()
		<-heartbeatDone
		<-conn.Done()
	}()
	watchDone := make(chan struct{})
	stopWatch := context.AfterFunc(ctx, func() {
		defer close(watchDone)
		client.mu.Lock()
		session := client.session
		client.mu.Unlock()
		if session != "" {
			cancelCtx, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			sent := make(chan struct{})
			go func() {
				defer close(sent)
				_ = conn.Cancel(cancelCtx, acp.CancelNotification{SessionId: session})
			}()
			select {
			case <-sent:
			case <-cancelCtx.Done():
			}
			_ = socket.CloseNow()
			<-sent
		} else {
			_ = socket.CloseNow()
		}
	})
	defer func() {
		if !stopWatch() {
			<-watchDone
		}
	}()
	err = e.acpTurn(ctx, conn, client, def.Cwd)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("harness: remote ACP: %w", err)
	}
	return nil
}

// dialACPWebSocket authenticates the handshake without exposing credentials in errors.
func dialACPWebSocket(ctx context.Context, def *ir.HarnessDefinition, client *http.Client) (*websocket.Conn, error) {
	headers := make(http.Header, len(def.Headers))
	for key, value := range def.Headers {
		headers.Set(key, value)
	}
	protocols := []string{"acp.v1"}
	if auth := strings.Fields(headers.Get("Authorization")); len(auth) == 2 && strings.EqualFold(auth[0], "Bearer") {
		// ACP UI bridges can authenticate the browser-compatible subprotocol.
		protocols = append(protocols, "bearer."+auth[1])
	}
	dialCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	socket, response, err := websocket.Dial(dialCtx, def.URL, &websocket.DialOptions{
		HTTPClient: client, HTTPHeader: headers, Subprotocols: protocols,
	})
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// Dial errors can include the URL, query parameters, or response body.
		if response != nil {
			return nil, fmt.Errorf("harness: ACP WebSocket connection failed (HTTP %d)", response.StatusCode)
		}
		return nil, errors.New("harness: ACP WebSocket connection failed")
	}
	socket.SetReadLimit(10 * 1024 * 1024)
	return socket, nil
}

// acpWebSocketStream adapts complete text frames to the SDK's NDJSON stream.
// A bridge may also batch multiple newline-delimited messages in one frame.
type acpWebSocketStream struct {
	ctx     context.Context
	conn    *websocket.Conn
	pending io.Reader
}

// Read appends a delimiter at each text frame boundary for the SDK's scanner.
func (s *acpWebSocketStream) Read(p []byte) (int, error) {
	for {
		if s.pending != nil {
			n, err := s.pending.Read(p)
			if err != io.EOF {
				return n, err
			}
			s.pending = nil
			if n > 0 {
				return n, nil
			}
		}
		kind, data, err := s.conn.Read(s.ctx)
		if err != nil {
			return 0, err
		}
		if kind != websocket.MessageText {
			return 0, errors.New("ACP WebSocket requires text frames")
		}
		s.pending = io.MultiReader(bytes.NewReader(data), strings.NewReader("\n"))
	}
}

// Write preserves the SDK's NDJSON delimiter for bridges that forward frames to stdin.
func (s *acpWebSocketStream) Write(p []byte) (int, error) {
	if err := s.conn.Write(s.ctx, websocket.MessageText, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

// keepACPWebSocketAlive uses ACP UI's notification heartbeat across idle proxies.
func keepACPWebSocketAlive(ctx context.Context, socket *websocket.Conn) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := socket.Write(pingCtx, websocket.MessageText, []byte("{\"jsonrpc\":\"2.0\",\"method\":\"$/ping\"}\n"))
			cancel()
			if err != nil {
				_ = socket.CloseNow()
				return
			}
		}
	}
}
