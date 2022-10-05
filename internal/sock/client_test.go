package sock

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDialFail(t *testing.T) {
	f, err := os.CreateTemp("", "sock_client_dial_failure")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	client := Client{Addr: f.Name()}
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
}

func TestDialTimeout(t *testing.T) {
	f, err := os.CreateTemp("", "sock_client_test")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(f.Name())
	}()

	s, err := NewServer(
		&Config{
			Addr: f.Name(),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(time.Second * 3100)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("OK"))
			},
		})
	require.NoError(t, err)

	go func() {
		_ = s.Serve(nil)
	}()

	time.Sleep(time.Millisecond * 500)

	require.NoError(t, err)
	client := Client{Addr: f.Name()}
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrTimeout))
}

func TestProcErr(t *testing.T) {
	e := procError("test", fmt.Errorf("error"))
	require.Contains(t, e.Error(), "test failed")

	e = procError("test", errTimeout)
	require.Contains(t, e.Error(), "test timeout")
}

type testTimeout struct{ error }

var errTimeout net.Error = &testTimeout{error: fmt.Errorf("timeout")}

func (t *testTimeout) Timeout() bool   { return true }
func (t *testTimeout) Temporary() bool { return false }
