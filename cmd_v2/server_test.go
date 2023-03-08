package cmd_v2

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerCommand(t *testing.T) {
	port := findPort(t)

	// Start the server.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, serverCommand(), cmdTest{
			args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			expectedOut: []string{"server is running"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 500)

	// Stop the server.
	res, err := http.Post(
		fmt.Sprintf("http://%s:%s/shutdown", "localhost", port),
		"application/json",
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	<-done
}

func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}
