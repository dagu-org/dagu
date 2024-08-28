// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package sock

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

var (
	ErrTimeout           = fmt.Errorf("unix socket timeout")
	ErrConnectionRefused = fmt.Errorf("unix socket connection failed")
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

// Request sends a request to the frontend and returns the response.
func (cl *Client) Request(method, url string) (string, error) {
	conn, err := net.DialTimeout("unix", cl.addr, defaultTimeout)
	if err != nil {
		return "", fmt.Errorf("dial failed: %w", err)
	}

	defer func() {
		_ = conn.Close()
	}()

	if err := conn.SetDeadline((time.Now().Add(defaultTimeout))); err != nil {
		return "", fmt.Errorf("set deadline failed: %w", err)
	}

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", fmt.Errorf("create request failed: %w", err)
	}

	if err := request.Write(conn); err != nil {
		return "", fmt.Errorf("write request failed: %w", err)
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		if err, ok := err.(net.Error); ok && err.Timeout() {
			return "", fmt.Errorf("request timeout: %w", ErrTimeout)
		}
		return "", fmt.Errorf("read response failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("read body failed: %w", err)
	}

	return string(body), nil
}
