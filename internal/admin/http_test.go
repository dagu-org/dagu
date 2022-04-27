package admin_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
)

func TestHttpServerStartShutdown(t *testing.T) {
	dir, err := ioutil.TempDir("", "test_http_server")
	require.NoError(t, err)
	os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort()
	server := admin.NewServer(&admin.Config{
		Host: host,
		Port: port,
	})

	go func() {
		err := server.Serve()
		require.Equal(t, http.ErrServerClosed, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	server.Shutdown()

	resp, err = http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.Error(t, err)
}

func TestHttpServerShutdownWithAPI(t *testing.T) {
	dir, err := ioutil.TempDir("", "test_http_server")
	require.NoError(t, err)
	os.RemoveAll(dir)

	host := "127.0.0.1"
	port := findPort()
	server := admin.NewServer(&admin.Config{
		Host: host,
		Port: port,
		Jobs: dir,
	})

	go func() {
		err := server.Serve()
		require.Equal(t, http.ErrServerClosed, err)
	}()

	time.Sleep(time.Millisecond * 300)

	resp, err := http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	resp, err = http.Post(fmt.Sprintf("http://%s:%s/shutdown", host, port), "", nil)
	require.NoError(t, err)
	require.Equal(t, "200 OK", resp.Status)

	time.Sleep(time.Millisecond * 1000)

	resp, err = http.Get(fmt.Sprintf("http://%s:%s", host, port))
	require.Error(t, err)
}

func findPort() string {
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
