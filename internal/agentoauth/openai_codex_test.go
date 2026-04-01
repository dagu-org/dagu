// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agentoauth

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAuthorizationInput(t *testing.T) {
	t.Parallel()

	t.Run("parses redirect URL", func(t *testing.T) {
		t.Parallel()

		code, state, err := parseAuthorizationInput(
			"http://localhost:1455/auth/callback?code=code-123&state=state-123",
			"",
		)
		require.NoError(t, err)
		assert.Equal(t, "code-123", code)
		assert.Equal(t, "state-123", state)
	})

	t.Run("parses code with inline state", func(t *testing.T) {
		t.Parallel()

		code, state, err := parseAuthorizationInput("", "code-123#state-123")
		require.NoError(t, err)
		assert.Equal(t, "code-123", code)
		assert.Equal(t, "state-123", state)
	})
}

func TestExtractAccountID(t *testing.T) {
	t.Parallel()

	headerJSON, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	require.NoError(t, err)
	payloadJSON, err := json.Marshal(map[string]any{
		openAICodexJWTClaimPath: map[string]any{
			"chatgpt_account_id": "acct-123",
		},
	})
	require.NoError(t, err)

	token := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(payloadJSON) + ".signature"

	accountID, err := extractAccountID(token)
	require.NoError(t, err)
	assert.Equal(t, "acct-123", accountID)
}
