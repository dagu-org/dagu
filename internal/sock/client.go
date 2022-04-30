package sock

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/yohamta/dagman/internal/utils"
)

var ErrTimeout = fmt.Errorf("unix socket timeout")
var ErrConnectionRefused = fmt.Errorf("unix socket connection failed")
var ErrFileNotExist = fmt.Errorf("unix socket file does not exit")
var timeout = time.Millisecond * 3000

type Client struct {
	Addr string
}

func (cl *Client) Request(method, url string) (string, error) {
	if !utils.FileExists(cl.Addr) {
		return "", fmt.Errorf("%w: %s", ErrFileNotExist, cl.Addr)
	}
	conn, err := net.DialTimeout("unix", cl.Addr, timeout)
	if err != nil {
		if err.(net.Error).Timeout() {
			return "", fmt.Errorf("%s: %w", err, ErrTimeout)
		} else {
			return "", fmt.Errorf("%s: %w", err, ErrConnectionRefused)
		}
	}
	defer conn.Close()
	err = conn.SetDeadline((time.Now().Add(timeout)))
	if err != nil {
		return "", err
	}
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Printf("NewRequest %v", err)
		return "", err
	}
	request.Write(conn)
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		if err.(net.Error).Timeout() {
			return "", fmt.Errorf("%s: %w", err, ErrTimeout)
		} else {
			return "", fmt.Errorf("failed to read: %w addr=%s", err, cl.Addr)
		}
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		if err.(net.Error).Timeout() {
			return "", fmt.Errorf("%s : %w", err, ErrTimeout)
		} else {
			return "", fmt.Errorf("failed to write: %w", err)
		}
	}
	return string(body), nil
}
