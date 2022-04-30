package sock_test

import (
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagman/internal/sock"
)

func TestDialFail(t *testing.T) {
	f, err := ioutil.TempFile("", "sock_client_dial_failure")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	client := sock.Client{Addr: f.Name()}
	_, err = client.Request("GET", "/status")
	assert.True(t, errors.Is(err, sock.ErrConnectionRefused))
}

func TestDialTimeout(t *testing.T) {
	f, err := ioutil.TempFile("", "sock_client_test")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	s, err := sock.NewServer(
		&sock.Config{
			Addr: f.Name(),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(time.Second * 3100)
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
		})
	require.NoError(t, err)

	go func() {
		s.Serve(nil)
	}()

	time.Sleep(time.Millisecond * 500)

	require.NoError(t, err)
	client := sock.Client{Addr: f.Name()}
	_, err = client.Request("GET", "/status")
	require.Error(t, err)
	assert.True(t, errors.Is(err, sock.ErrTimeout))
}
