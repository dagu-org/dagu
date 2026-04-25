// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPHandlerRecoversFromHandlerPanic(t *testing.T) {
	t.Parallel()

	srv, err := NewServer(
		"ignored",
		func(http.ResponseWriter, *http.Request) {
			panic("boom")
		},
	)
	require.NoError(t, err)

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	recorder := httptest.NewRecorder()

	require.NotPanics(t, func() {
		srv.httpHandler(context.Background()).ServeHTTP(recorder, request)
	})
	require.Equal(t, http.StatusInternalServerError, recorder.Code)
}
