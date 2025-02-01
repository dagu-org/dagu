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
