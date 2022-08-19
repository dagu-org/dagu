package main

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

type serverTestExpect struct {
	host string
	port string
}

func Test_serverCommand(t *testing.T) {

	app := makeApp()
	dir := utils.MustTempDir("dagu_test_server")
	os.Setenv("HOME", dir)
	settings.ChangeHomeDir(dir)
	defCfg, err := admin.DefaultConfig()

	port := findPort(t)
	settings.Set(settings.SETTING__ADMIN_PORT, port)

	for _, tc := range []struct {
		args   []string
		expect serverTestExpect
	}{
		{
			args: []string{"", "server"},
			expect: serverTestExpect{
				port: port,
				host: defCfg.Host,
			},
		},
		{
			args: []string{"", "server",
				fmt.Sprintf("--port=%s", port),
				fmt.Sprintf("--host=%s", "localhost"),
			},
			expect: serverTestExpect{
				port: port,
				host: "localhost",
			},
		},
	} {
		done := make(chan struct{})
		go func() {
			runAppTestOutput(app, appTest{
				args: tc.args, errored: false,
				output: []string{"admin server is running "},
			}, t)
			close(done)
		}()

		time.Sleep(time.Millisecond * 100)
		require.NoError(t, err)

		res, err := http.Post(
			fmt.Sprintf("http://%s:%s/shutdown", tc.expect.host, tc.expect.port),
			"application/json",
			nil,
		)

		require.NoError(t, err)
		require.Equal(t, http.StatusOK, res.StatusCode)

		<-done
	}

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
