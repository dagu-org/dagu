package admin

import (
	"fmt"
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
	os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort(t)
	server := NewServer(&Config{
		Host: host,
		Port: port,
		DAGs: testTempDir,
	})

	go func() {
		err := server.Serve()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	server.Shutdown()

	_, err = http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.Error(t, err)
}

func TestHttpServerShutdownWithAPI(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_http_server")
	require.NoError(t, err)
	os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort(t)
	server := NewServer(&Config{
		Host: host,
		Port: port,
		DAGs: dir,
	})

	go func() {
		err := server.Serve()
		require.NoError(t, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	resp, err = http.Post(fmt.Sprintf("http://%s:%s/shutdown", host, port), "", nil)
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	time.Sleep(time.Millisecond * 1000)

	_, err = http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.Error(t, err)
}

func TestHttpServerBasicAuth(t *testing.T) {
	dir, err := os.MkdirTemp("", "test_http_server")
	require.NoError(t, err)
	os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort(t)
	server := NewServer(&Config{
		Host:              host,
		Port:              port,
		IsBasicAuth:       true,
		BasicAuthUsername: "user",
		BasicAuthPassword: "password",
		DAGs:              testTempDir,
	})

	go func() {
		err := server.Serve()
		require.NoError(t, err)
	}()
	defer server.Shutdown()

	time.Sleep(time.Millisecond * 300)

	client := &http.Client{
		Timeout: time.Second * 1,
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s:%s", host, port), nil)
	require.NoError(t, err)

	res, err := client.Do(req)
	require.NoError(t, err)
	require.Equal(t, "401 Unauthorized", res.Status)

	req, err = http.NewRequest("GET", fmt.Sprintf("http://%s:%s", host, port), nil)
	require.NoError(t, err)
	req.SetBasicAuth("user", "password")

	res, err = client.Do(req)
	require.NoError(t, err)
	require.Equal(t, "200 OK", res.Status)
}

func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d", port)
}
