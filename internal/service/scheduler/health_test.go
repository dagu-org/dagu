// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHealthServer(t *testing.T) {
	t.Run("StartStop", func(t *testing.T) {
		hs := newHealthServerWithAddr("127.0.0.1:0")
		ctx := context.Background()

		// Start the server
		err := hs.Start(ctx)
		require.NoError(t, err)

		// Make a request to the health endpoint
		resp, err := http.Get(hs.URL() + "/health")
		require.NoError(t, err)
		defer func() {
			err := resp.Body.Close()
			assert.NoError(t, err)
		}()

		// Check the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

		var healthResp HealthResponse
		err = json.NewDecoder(resp.Body).Decode(&healthResp)
		require.NoError(t, err)
		assert.Equal(t, "healthy", healthResp.Status)

		// Stop the server
		err = hs.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("DisabledWhenPortIsZero", func(t *testing.T) {
		hs := NewHealthServer(0)
		ctx := context.Background()

		// Start should succeed but not actually start a server
		err := hs.Start(ctx)
		require.NoError(t, err)

		// Stop should also succeed
		err = hs.Stop(ctx)
		require.NoError(t, err)
	})
}
