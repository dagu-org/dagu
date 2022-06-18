package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

func Test_serverCommand(t *testing.T) {
	app := makeApp()
	dir := utils.MustTempDir("dagu_test_server")
	os.Setenv("HOME", dir)

	port := findPort(t)
	os.Setenv(settings.SETTING__ADMIN_PORT, port)
	settings.ChangeHomeDir(dir)

	done := make(chan struct{})
	go func() {
		runAppTestOutput(app, appTest{
			args: []string{"", "server"}, errored: false,
			output: []string{"admin server is running "},
		}, t)
		close(done)
	}()

	time.Sleep(time.Millisecond * 100)

	cfg := admin.DefaultConfig()
	res, err := http.Post(
		fmt.Sprintf("http://%s:%s/shutdown", cfg.Host, cfg.Port),
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
	if err != nil {
		panic(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		panic(err)
	}
	return fmt.Sprintf("%d", port)
}
