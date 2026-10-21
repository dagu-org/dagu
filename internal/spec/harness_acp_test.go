// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestACPDefinition checks protocol selection and incompatible CLI prompt options.
func TestACPDefinition(t *testing.T) {
	def, err := parseHarnessDefinition("agent", map[string]any{"binary": "adapter", "protocol": "acp", "prefix_args": []any{"--acp"}})
	require.NoError(t, err)
	require.Equal(t, "acp", def.Protocol)
	require.Equal(t, []string{"--acp"}, def.PrefixArgs)
	for _, raw := range []map[string]any{
		{"binary": "adapter", "protocol": "http"},
		{"binary": "adapter", "protocol": true},
		{"binary": "adapter", "protocol": "acp", "prompt_mode": "stdin"},
		{"binary": "adapter", "protocol": "acp", "prompt_flag": "-p"},
		{"binary": "adapter", "protocol": "acp", "prompt_position": "after_flags"},
	} {
		_, err := parseHarnessDefinition("agent", raw)
		require.Error(t, err)
	}
}
