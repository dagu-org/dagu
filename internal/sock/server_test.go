// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package sock_test

import (
	"errors"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/sock"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	testHomeDir, err := os.MkdirTemp("", "controller_test")
	if err != nil {
		panic(err)
	}
	_ = os.Setenv("HOME", testHomeDir)
	code := m.Run()
	_ = os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func TestStartAndShutdownServer(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_server_start_shutdown")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		},
		test.NewLogger(),
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error)
	go func() {
		for range listen {
		}
	}()

	go func() {
		err := unixServer.Serve(listen)
		require.True(t, errors.Is(err, sock.ErrServerRequestedShutdown))
	}()

	time.Sleep(time.Millisecond * 50)

	ret, err := client.Request(http.MethodPost, "/")
	require.NoError(t, err)
	require.Equal(t, "OK", ret)

	_ = unixServer.Shutdown()

	time.Sleep(time.Millisecond * 50)
	_, err = client.Request(http.MethodPost, "/")
	require.Error(t, err)
}

func TestNoResponse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_error_response")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		},
		test.NewLogger(),
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error)
	go func() {
		for range listen {
		}
	}()

	go func() {
		err = unixServer.Serve(listen)
		_ = unixServer.Shutdown()
	}()

	time.Sleep(time.Millisecond * 50)

	_, err = client.Request(http.MethodGet, "/")
	require.Error(t, err)
}

func TestErrorResponse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_error_response")
	require.NoError(t, err)
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(_ http.ResponseWriter, _ *http.Request) {},
		test.NewLogger(),
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error)
	go func() {
		for range listen {
		}
	}()

	go func() {
		err = unixServer.Serve(listen)
		_ = unixServer.Shutdown()
	}()

	time.Sleep(time.Millisecond * 50)

	_, err = client.Request(http.MethodGet, "/")
	require.Error(t, err)
}
