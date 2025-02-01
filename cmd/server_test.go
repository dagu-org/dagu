package main

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestServerCommand(t *testing.T) {
	t.Run("StartServer", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			th.RunCommand(t, serverCmd(), cmdTest{
				args:        []string{"server", fmt.Sprintf("--port=%s", findPort(t))},
				expectedOut: []string{"server is running"},
			})
		}()

		time.Sleep(time.Millisecond * 500)
	})
}

// findPort finds an available port.
func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}
