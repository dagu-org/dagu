package sock

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

var ErrTimeout = fmt.Errorf("unix socket timeout")
var ErrConnectionRefused = fmt.Errorf("unix socket connection failed")
var timeout = time.Millisecond * 3000

type Client struct {
	Addr string
}

func (cl *Client) Request(method, url string) (string, error) {
	conn, err := net.DialTimeout("unix", cl.Addr, timeout)
	if err != nil {
		return "", procError("dial to socket", err)
	}
	defer conn.Close()
	conn.SetDeadline((time.Now().Add(timeout)))
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Printf("NewRequest %v", err)
		return "", err
	}
	request.Write(conn)
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return "", procError("read response", err)
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", procError("read response body", err)
	}
	return string(body), nil
}

func procError(action string, err error) error {
	if err, ok := err.(net.Error); ok {
		if err.Timeout() {
			return fmt.Errorf("%s timeout %w: %s", action, ErrTimeout, err.Error())
		}
	}
	return fmt.Errorf("%s failed: %w", action, err)
}
