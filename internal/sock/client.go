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

var timeout = time.Millisecond * 3000

// Request sends a request to the frontend and returns the response.
func (cl *Client) Request(method, url string) (string, error) {
	conn, err := net.DialTimeout("unix", cl.addr, timeout)
	if err != nil {
		return "", handleError("dial to socket", err)
	}

	defer func() {
		_ = conn.Close()
	}()

	_ = conn.SetDeadline((time.Now().Add(timeout)))

	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		return "", handleError("new request", err)
	}

	_ = request.Write(conn)

	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return "", handleError("read response", err)
	}

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", handleError("read response body", err)
	}

	return string(body), nil
}

func handleError(action string, err error) error {
	if err, ok := err.(net.Error); ok && err.Timeout() {
		return fmt.Errorf("%s timeout %w: %s", action, ErrTimeout, err.Error())
	}
	return fmt.Errorf("%s failed: %w", action, err)
}
