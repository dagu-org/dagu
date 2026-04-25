// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServeConnRecoversFromHandlerPanic(t *testing.T) {
	t.Parallel()

	serverConn, clientConn := net.Pipe()
	defer func() {
		_ = clientConn.Close()
	}()

	srv, err := NewServer(
		"ignored",
		func(http.ResponseWriter, *http.Request) {
			panic("boom")
		},
	)
	require.NoError(t, err)

	request, err := http.NewRequest(http.MethodGet, "/", nil)
	require.NoError(t, err)

	go func() {
		_ = request.Write(clientConn)
		_ = clientConn.Close()
	}()

	srv.connWG.Add(1)
	require.NotPanics(t, func() {
		srv.serveConn(context.Background(), serverConn)
	})
}
