package sock_test

import (
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/jobctl/internal/sock"
	"github.com/yohamta/jobctl/internal/utils"
)

var (
	testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")
)

func TestMain(m *testing.M) {
	testHomeDir, err := ioutil.TempDir("", "controller_test")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", testHomeDir)
	code := m.Run()
	os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func TestStartServer(t *testing.T) {
	tmpFile, err := ioutil.TempFile("", "test-server-start")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	unixServer, err := sock.NewServer(
		&sock.Config{
			Addr: tmpFile.Name(),
			HandlerFunc: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("OK"))
			},
		})
	require.NoError(t, err)

	client, err := sock.NewUnixClient(tmpFile.Name())
	require.NoError(t, err)

	listen := make(chan error)
	go func() {
		for range listen {
		}
	}()

	go func() {
		err = unixServer.Serve(listen)
		require.NoError(t, err)
	}()

	time.Sleep(time.Second * 1)

	ret, err := client.Request(http.MethodPost, "/")
	assert.Equal(t, ret, "OK")
}
