package sock

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
)

type Client struct {
	addr *net.UnixAddr
}

func NewUnixClient(nw string) (*Client, error) {
	addr, err := net.ResolveUnixAddr("unix", nw)
	if err != nil {
		return nil, err
	}
	return &Client{
		addr: addr,
	}, nil
}

func (cl *Client) Request(method, url string) (string, error) {
	conn, err := net.DialUnix("unix", nil, cl.addr)
	if err != nil {
		return "", fmt.Errorf("the job is not running")
	}
	defer conn.Close()
	request, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Printf("NewRequest %v", err)
		return "", err
	}
	request.Write(conn)
	response, err := http.ReadResponse(bufio.NewReader(conn), request)
	if err != nil {
		return "", err
	}
	body, err := io.ReadAll(response.Body)
	if err != nil {
		log.Printf("ReadAll %v", err)
		return "", err
	}
	return string(body), nil
}
