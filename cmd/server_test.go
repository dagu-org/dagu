package cmd

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestServerCommand(t *testing.T) {
	port := findPort(t)

	// Start the server.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, serverCmd(), cmdTest{
			args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			expectedOut: []string{"server is running"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)

	// Stop the server.
	res, err := http.Post(
		fmt.Sprintf("http://%s/shutdown", net.JoinHostPort("localhost", port)),
		"application/json",
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	<-done
}
