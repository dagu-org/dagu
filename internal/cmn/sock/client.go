// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	ErrTimeout = errors.New("unix socket timeout")
)

// Client is a unix socket client that can send requests
// to the frontend over HTTP.
type Client struct {
	addr string
}

func NewClient(addr string) *Client {
	return &Client{addr: addr}
}

const defaultTimeout = 3 * time.Second

func wrapTimeout(op string, err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("%s: %w", op, ErrTimeout)
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%s: %w", op, ErrTimeout)
	}

	return fmt.Errorf("%s: %w", op, err)
}

func normalizePath(path string) string {
	if path == "" {
		return "/"
	}
	if strings.HasPrefix(path, "/") {
		return path
	}
	return "/" + path
}

func (cl *Client) httpClient() *http.Client {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: defaultTimeout}
			conn, err := dialer.DialContext(ctx, "unix", cl.addr)
			if err != nil {
				return nil, wrapTimeout("dial unix socket", err)
			}
			return conn, nil
		},
		DisableCompression: true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   defaultTimeout,
	}
}

// Request sends a request to the frontend and returns the response.
func (cl *Client) Request(method, path string) (string, error) {
	client := cl.httpClient()
	defer client.CloseIdleConnections()

	requestURL := &url.URL{
		Scheme: "http",
		Host:   "unix",
		Path:   normalizePath(path),
	}
	request, err := http.NewRequest(method, requestURL.String(), nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return "", wrapTimeout("send request", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", wrapTimeout("read response body", err)
	}

	return string(body), nil
}

// SocketAddr returns the address of the unix socket.
func (cl *Client) SocketAddr() string {
	return cl.addr
}
