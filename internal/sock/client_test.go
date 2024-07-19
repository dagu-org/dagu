package sock_test

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/sock"
	"github.com/dagu-dev/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDialFail(t *testing.T) {
	f, err := os.CreateTemp("", "sock_client_dial_failure")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	client := sock.NewClient(f.Name())
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
}

func TestDialTimeout(t *testing.T) {
	f, err := os.CreateTemp("", "sock_client_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	srv, err := sock.NewServer(
		f.Name(),
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(time.Second * 3100)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		},
		test.NewLogger(),
	)
	require.NoError(t, err)

	go func() {
		_ = srv.Serve(nil)
	}()

	time.Sleep(time.Millisecond * 500)

	require.NoError(t, err)
	client := sock.NewClient(f.Name())
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
	require.True(t, errors.Is(err, sock.ErrTimeout))
}

type testTimeout struct{ error }

var errTimeout net.Error = &testTimeout{error: fmt.Errorf("timeout")}

func (t *testTimeout) Timeout() bool   { return true }
func (t *testTimeout) Temporary() bool { return false }
