// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
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

const (
	defaultTimeout = time.Millisecond * 3000
)

func wrapTimeout(op string, err error) error {
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return fmt.Errorf("%s: %w", op, ErrTimeout)
	}
	return fmt.Errorf("%s: %w", op, err)
}

// Request sends a request to the frontend and returns the response.
func (cl *Client) Request(method, url string) (string, error) {
	conn, err := net.DialTimeout("unix", cl.addr, defaultTimeout)
	if err != nil {
		return "", fmt.Errorf("dial unix socket: %w", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	if err := conn.SetDeadline(time.Now().Add(defaultTimeout)); err != nil {
		return "", fmt.Errorf("set connection deadline: %w", err)
	}

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	if err := request.Write(conn); err != nil {
		return "", wrapTimeout("write request", err)
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return "", wrapTimeout("read response", err)
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
