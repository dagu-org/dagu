// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock_test

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/sock"
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
	require.NoError(t, tmpFile.Close())
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		},
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error, 1)

	go func() {
		err := unixServer.Serve(context.Background(), listen)
		require.True(t, errors.Is(err, sock.ErrServerRequestedShutdown))
	}()

	// Wait for the server to signal it is ready.
	require.NoError(t, <-listen)

	ret, err := client.Request(http.MethodPost, "/")
	require.NoError(t, err)
	require.Equal(t, "OK", ret)

	_ = unixServer.Shutdown(context.Background())

	// Wait for the server to finish shutting down.
	require.Eventually(t, func() bool {
		_, err := client.Request(http.MethodPost, "/")
		return err != nil
	}, 5*time.Second, 10*time.Millisecond)
}

func TestNoResponse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_error_response")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		},
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error, 1)

	go func() {
		err = unixServer.Serve(context.Background(), listen)
		_ = unixServer.Shutdown(context.Background())
	}()

	// Wait for the server to signal it is ready.
	require.NoError(t, <-listen)

	_, err = client.Request(http.MethodGet, "/")
	require.Error(t, err)
}

func TestErrorResponse(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_error_response")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(_ http.ResponseWriter, _ *http.Request) {},
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error, 1)

	go func() {
		err = unixServer.Serve(context.Background(), listen)
		_ = unixServer.Shutdown(context.Background())
	}()

	// Wait for the server to signal it is ready.
	require.NoError(t, <-listen)

	_, err = client.Request(http.MethodGet, "/")
	require.Error(t, err)
}

func TestShutdownWhileServerStarts(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_shutdown_while_server_starts")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		},
	)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		done <- unixServer.Serve(context.Background(), nil)
	}()

	require.NoError(t, unixServer.Shutdown(context.Background()))

	select {
	case err := <-done:
		require.True(t, errors.Is(err, sock.ErrServerRequestedShutdown))
	case <-time.After(func() time.Duration {
		if runtime.GOOS == "windows" {
			return 5 * time.Second
		}
		return time.Second
	}()):
		t.Fatal("timed out waiting for socket server to stop")
	}
}

func TestMultipleWritesProduceSingleResponseBody(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test_server_multiple_writes")
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())
	defer func() {
		_ = os.Remove(tmpFile.Name())
	}()

	unixServer, err := sock.NewServer(
		tmpFile.Name(),
		func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("O"))
			_, _ = w.Write([]byte("K"))
		},
	)
	require.NoError(t, err)

	client := sock.NewClient(tmpFile.Name())
	listen := make(chan error, 1)
	done := make(chan error, 1)

	go func() {
		done <- unixServer.Serve(context.Background(), listen)
	}()

	require.NoError(t, <-listen)

	body, err := client.Request(http.MethodGet, "/")
	require.NoError(t, err)
	require.Equal(t, "OK", body)

	require.NoError(t, unixServer.Shutdown(context.Background()))
	require.True(t, errors.Is(<-done, sock.ErrServerRequestedShutdown))
}
