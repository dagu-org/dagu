// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sock

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestNewClientInitializesReusableHTTPClient verifies the cached HTTP client setup.
func TestNewClientInitializesReusableHTTPClient(t *testing.T) {
	t.Parallel()

	client := NewClient("unix.sock")

	require.NotNil(t, client.client)
	require.Equal(t, defaultTimeout, client.client.Timeout)
	require.NotNil(t, client.client.Transport)
}
