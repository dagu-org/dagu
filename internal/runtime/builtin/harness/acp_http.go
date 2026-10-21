// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
)

const acpMessageLimit = 10 * 1024 * 1024

// acpRemoteHTTPClient keeps TLS verification enabled, optionally adding private CA roots.
func acpRemoteHTTPClient(caFile, workDir string) (*http.Client, error) {
	if caFile == "" {
		return acpRemoteDefaultHTTPClient, nil
	}
	if !filepath.IsAbs(caFile) {
		caFile = filepath.Join(workDir, caFile)
	}
	pem, err := os.ReadFile(caFile) // #nosec G304 -- explicitly configured local CA file.
	if err != nil {
		return nil, errors.New("harness: cannot read ACP ca_file")
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, errors.New("harness: cannot load system certificate roots")
	}
	if !roots.AppendCertsFromPEM(pem) {
		return nil, errors.New("harness: ACP ca_file contains no certificates")
	}
	transport, ok := newOpenCodeTransport().(*http.Transport)
	if !ok {
		return nil, errors.New("harness: configured HTTP transport cannot use ca_file")
	}
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	client := *acpRemoteDefaultHTTPClient
	client.Transport = transport
	return &client, nil
}

// acpHTTPStream implements ACP's POST/GET/DELETE profile, not MCP's POST response stream.
// initialize returns JSON; subsequent responses and callbacks arrive on scoped SSE GETs.
type acpHTTPStream struct {
	ctx        context.Context
	cancel     context.CancelFunc
	def        *ir.HarnessDefinition
	client     *http.Client
	incoming   chan []byte
	pending    *bytes.Reader
	mu         sync.Mutex
	connection string
	sessions   map[string]bool
	routes     map[string]string
	err        error
	closing    bool
	wg         sync.WaitGroup
	once       sync.Once
}

// newACPHTTPStream isolates affinity cookies and connection state for one execution.
func newACPHTTPStream(ctx context.Context, def *ir.HarnessDefinition, base *http.Client) *acpHTTPStream {
	ctx, cancel := context.WithCancel(ctx)
	client := *base
	client.Jar, _ = cookiejar.New(nil)
	return &acpHTTPStream{ctx: ctx, cancel: cancel, def: def, client: &client,
		incoming: make(chan []byte, 64), sessions: make(map[string]bool), routes: make(map[string]string)}
}

type acpHTTPMessage struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params struct {
		SessionID string `json:"sessionId"`
	} `json:"params"`
	Result struct {
		SessionID string `json:"sessionId"`
	} `json:"result"`
}

// Read adapts scoped SSE messages to the SDK's ordered NDJSON input.
func (s *acpHTTPStream) Read(p []byte) (int, error) {
	for {
		if s.pending != nil {
			n, err := s.pending.Read(p)
			if err != io.EOF {
				return n, err
			}
			s.pending = nil
		}
		// Drain already received responses before reporting a closed stream.
		select {
		case data := <-s.incoming:
			s.pending = bytes.NewReader(data)
			continue
		default:
		}
		select {
		case data := <-s.incoming:
			s.pending = bytes.NewReader(data)
		case <-s.ctx.Done():
			s.mu.Lock()
			err := s.err
			s.mu.Unlock()
			if err == nil {
				err = io.EOF
			}
			return 0, err
		}
	}
}

// Write posts one SDK message and opens the required connection/session SSE streams.
func (s *acpHTTPStream) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	if s.closing || s.ctx.Err() != nil {
		s.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	s.wg.Add(1)
	connection := s.connection
	s.mu.Unlock()
	defer s.wg.Done()
	defer func() {
		if err != nil {
			s.fail(err)
		}
	}()
	var msg acpHTTPMessage
	if len(p) > acpMessageLimit || json.Unmarshal(p, &msg) != nil {
		return 0, errors.New("invalid ACP HTTP message")
	}
	session := msg.Params.SessionID
	if msg.Method == "" {
		s.mu.Lock()
		session = s.routes[string(msg.ID)]
		s.mu.Unlock()
	}
	if session != "" {
		if err := s.openSSE(session); err != nil {
			return 0, err
		}
	}
	ctx, cancel := context.WithTimeout(s.ctx, 15*time.Second)
	defer cancel()
	response, err := s.request(ctx, http.MethodPost, session, p)
	if err != nil {
		return 0, err
	}
	defer func() { _ = response.Body.Close() }()
	if connection == "" {
		if msg.Method != "initialize" || response.StatusCode != http.StatusOK {
			return 0, errors.New("ACP HTTP requires a successful initialize response")
		}
		id := response.Header.Get("Acp-Connection-Id")
		if id == "" || len(id) > 1024 {
			return 0, errors.New("ACP HTTP initialize missing valid Acp-Connection-Id")
		}
		s.mu.Lock()
		s.connection = id
		s.mu.Unlock()
		data, err := io.ReadAll(io.LimitReader(response.Body, acpMessageLimit+1))
		var reply acpHTTPMessage
		if err != nil || len(data) > acpMessageLimit || json.Unmarshal(data, &reply) != nil || !bytes.Equal(reply.ID, msg.ID) {
			return 0, errors.New("invalid ACP HTTP initialize response")
		}
		if err := s.openSSE(""); err != nil {
			return 0, err
		}
		if err := s.deliver(data, ""); err != nil {
			return 0, err
		}
	} else if response.StatusCode != http.StatusAccepted {
		return 0, errors.New("ACP HTTP POST must return 202 Accepted")
	}
	if msg.Method == "" {
		s.mu.Lock()
		delete(s.routes, string(msg.ID))
		s.mu.Unlock()
	}
	return len(p), nil
}

// request preserves auth/cookies while protecting transport headers and error messages.
func (s *acpHTTPStream) request(ctx context.Context, method, session string, body []byte) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.def.URL, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("invalid ACP HTTP endpoint")
	}
	for key, value := range s.def.Headers {
		req.Header.Set(key, value)
	}
	req.Header.Del("Acp-Connection-Id")
	req.Header.Del("Acp-Session-Id")
	s.mu.Lock()
	connection := s.connection
	s.mu.Unlock()
	if connection != "" {
		req.Header.Set("Acp-Connection-Id", connection)
	}
	if session != "" {
		req.Header.Set("Acp-Session-Id", session)
	}
	if method == http.MethodGet {
		req.Header.Set("Accept", "text/event-stream")
	} else {
		req.Header.Set("Content-Type", "application/json")
	}
	response, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("ACP HTTP request failed")
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_ = response.Body.Close()
		return nil, fmt.Errorf("ACP HTTP request failed (HTTP %d)", response.StatusCode)
	}
	return response, nil
}

// openSSE subscribes before exposing new sessions to the SDK, avoiding lost updates.
func (s *acpHTTPStream) openSSE(session string) error {
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		return io.ErrClosedPipe
	}
	if s.sessions[session] {
		s.mu.Unlock()
		return nil
	}
	s.sessions[session] = true
	s.mu.Unlock()
	ctx, cancel := context.WithCancel(s.ctx)
	timer := time.AfterFunc(15*time.Second, cancel)
	response, err := s.request(ctx, http.MethodGet, session, nil)
	timer.Stop()
	if err != nil {
		cancel()
		return err
	}
	if strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0]) != "text/event-stream" {
		cancel()
		_ = response.Body.Close()
		return errors.New("ACP HTTP GET must return text/event-stream")
	}
	s.mu.Lock()
	if s.closing {
		s.mu.Unlock()
		cancel()
		_ = response.Body.Close()
		return io.ErrClosedPipe
	}
	s.wg.Add(1)
	s.mu.Unlock()
	go func() {
		defer s.wg.Done()
		defer cancel()
		defer func() { _ = response.Body.Close() }()
		s.fail(s.readSSE(response.Body, session))
	}()
	return nil
}

// readSSE supports comments and multiline data, with bounded event sizes.
func (s *acpHTTPStream) readSSE(r io.Reader, session string) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), acpMessageLimit)
	var data bytes.Buffer
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if data.Len() > 0 {
				if err := s.deliver(data.Bytes(), session); err != nil {
					return err
				}
				data.Reset()
			}
		} else if value, ok := strings.CutPrefix(line, "data:"); ok {
			if data.Len()+len(value)+1 > acpMessageLimit {
				return errors.New("ACP SSE event exceeds message limit")
			}
			data.WriteString(strings.TrimPrefix(value, " "))
			data.WriteByte('\n')
		}
	}
	return errors.New("ACP SSE stream disconnected")
}

// deliver routes server callbacks and compacts JSON for the Go SDK's NDJSON reader.
func (s *acpHTTPStream) deliver(data []byte, session string) error {
	var msg acpHTTPMessage
	var compact bytes.Buffer
	if len(data) > acpMessageLimit || json.Unmarshal(data, &msg) != nil || json.Compact(&compact, data) != nil {
		return errors.New("invalid ACP SSE message")
	}
	if msg.Result.SessionID != "" {
		if err := s.openSSE(msg.Result.SessionID); err != nil {
			return err
		}
	}
	if msg.Method != "" && len(msg.ID) > 0 {
		s.mu.Lock()
		s.routes[string(msg.ID)] = session
		s.mu.Unlock()
	}
	select {
	case s.incoming <- append(compact.Bytes(), '\n'):
		return nil
	case <-s.ctx.Done():
		return io.ErrClosedPipe
	}
}

// fail ends all scoped streams together; the execution owner performs final DELETE.
func (s *acpHTTPStream) fail(err error) {
	s.mu.Lock()
	if s.err == nil {
		s.err = err
	}
	s.mu.Unlock()
	s.cancel()
}

// Close aborts local I/O before sending a bounded DELETE to release the connection.
func (s *acpHTTPStream) Close() error {
	s.once.Do(func() {
		s.mu.Lock()
		s.closing = true
		s.mu.Unlock()
		s.cancel()
		s.wg.Wait()
		s.mu.Lock()
		connection := s.connection
		s.mu.Unlock()
		if connection != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if response, err := s.request(ctx, http.MethodDelete, "", nil); err == nil {
				_ = response.Body.Close()
			}
		}
	})
	return nil
}
