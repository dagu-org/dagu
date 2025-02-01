package sock_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/sock"
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
		func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(time.Second * 3100)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		},
	)
	require.NoError(t, err)

	go func() {
		_ = srv.Serve(context.Background(), nil)
	}()

	time.Sleep(time.Millisecond * 500)

	require.NoError(t, err)
	client := sock.NewClient(f.Name())
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
	require.True(t, errors.Is(err, sock.ErrTimeout))
}
