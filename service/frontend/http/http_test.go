package http

import (
	"context"
	"fmt"
	"github.com/yohamta/dagu/internal/logger"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHttpServerStartShutdown(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_http_server")
	require.NoError(t, err)
	_ = os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort(t)
	server := NewServer(ServerParams{Host: host, Port: port, Logger: logger.NewSlogLogger()})

	go func() {
		err := server.Start()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%d", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	server.Shutdown(context.Background())

	_, err = http.Get(fmt.Sprintf("http://%s:%d", host, port))
	require.Error(t, err)
}

func TestHttpServerShutdownWithAPI(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_http_server")
	require.NoError(t, err)
	_ = os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort(t)
	server := NewServer(ServerParams{Host: host, Port: port, Logger: logger.NewSlogLogger()})

	go func() {
		err := server.Start()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%d", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	resp, err = http.Post(fmt.Sprintf("http://%s:%d/shutdown", host, port), "", nil)
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	time.Sleep(time.Millisecond * 1000)

	_, err = http.Get(fmt.Sprintf("http://%s:%d", host, port))
	require.Error(t, err)
}

func findPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		panic(err)
	}
	return port
}
