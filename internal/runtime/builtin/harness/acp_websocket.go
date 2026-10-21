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
	"golang.org/x/net/http/httpguts"
)

var acpRemoteDefaultHTTPClient = &http.Client{
	Transport: newOpenCodeTransport(),
	// Remote endpoints must not forward credentials through redirects.
	CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
}

// runACPRemote shares session, permission, output, and cancellation handling across remote transports.
// The working directory is passed to the agent unchanged, never created locally.
func (e *harnessExecutor) runACPRemote(parent context.Context, cfg providerConfig, out io.Writer, allow bool) error {
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
		CAFile  string
		Cwd     string
		Headers map[string]string
	}{cfg.definition.URL, cfg.definition.CAFile, cfg.definition.Cwd, cfg.definition.Headers})
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("harness: could not resolve remote ACP connection settings")
	}
	def := cloneDefinition(cfg.definition)
	def.URL, def.Cwd, def.Headers = settings.URL, settings.Cwd, settings.Headers
	def.CAFile = settings.CAFile
	if err := def.ValidateConnection(); err != nil {
		return fmt.Errorf("harness: remote ACP: %w", err)
	}
	if strings.Contains(def.URL, "${") || strings.Contains(def.Cwd, "${") {
		return errors.New("harness: unresolved remote ACP connection settings")
	}
	httpClient, err := acpRemoteHTTPClient(settings.CAFile, e.workDir)
	if err != nil {
		return err
	}
	if def.CAFile != "" {
		defer httpClient.CloseIdleConnections()
	}
	wireCtx, closeWire := context.WithCancel(context.Background())
	defer closeWire()
	var stream io.ReadWriteCloser
	if def.Transport == ir.HarnessTransportHTTP {
		stream = newACPHTTPStream(wireCtx, def, httpClient)
	} else {
		socket, dialErr := dialACPWebSocket(ctx, def, httpClient)
		if dialErr != nil {
			return dialErr
		}
		stream = &acpWebSocketStream{ctx: wireCtx, conn: socket}
		go keepACPWebSocketAlive(wireCtx, socket, 25*time.Second)
	}
	client := &acpHarnessClient{out: out, allow: allow, progress: e.progressCallback}
	conn := acp.NewClientSideConnection(client, stream, stream)
	defer func() {
		closeWire()
		_ = stream.Close()
		<-conn.Done()
	}()
	defer watchACPCancellation(ctx, conn, client, func() { _ = stream.Close() })()
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
	if def.BearerSubprotocol {
		auth := strings.Fields(headers.Get("Authorization"))
		if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") || !httpguts.ValidHeaderFieldName("bearer."+auth[1]) {
			return nil, errors.New("harness: bearer_subprotocol requires a Bearer token valid as an HTTP token")
		}
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
	socket.SetReadLimit(acpMessageLimit)
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

// Close interrupts blocked reads and writes without waiting for a peer handshake.
func (s *acpWebSocketStream) Close() error { return s.conn.CloseNow() }

// keepACPWebSocketAlive uses protocol-level ping frames across idle proxies.
func keepACPWebSocketAlive(ctx context.Context, socket *websocket.Conn, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := socket.Ping(pingCtx)
			cancel()
			if err != nil {
				_ = socket.CloseNow()
				return
			}
		}
	}
}
