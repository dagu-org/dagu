package cmd

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestServerCommand(t *testing.T) {
	port := findPort(t)

	// Start the frontend.
	go func() {
		testRunCommand(t, serverCmd(), cmdTest{
			args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			expectedOut: []string{"server is running"},
		})
	}()

	time.Sleep(time.Millisecond * 300)
}

func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}
