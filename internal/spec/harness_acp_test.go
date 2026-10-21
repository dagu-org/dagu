// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"maps"
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

// TestACPRemoteDefinition checks remote endpoints without requiring a local binary.
func TestACPRemoteDefinition(t *testing.T) {
	raw := map[string]any{"protocol": "acp", "transport": "websocket", "url": "wss://agent.example/acp", "cwd": "/remote/workspace", "headers": map[string]any{"Authorization": "Bearer ${ACP_TOKEN}"}}
	def, err := parseHarnessDefinition("remote", raw)
	require.NoError(t, err)
	require.Empty(t, def.Binary)
	require.Equal(t, "websocket", def.Transport)
	require.Equal(t, "Bearer ${ACP_TOKEN}", def.Headers["Authorization"])
	httpConfig := maps.Clone(raw)
	httpConfig["transport"], httpConfig["url"] = "http", "https://agent.example/acp"
	_, err = parseHarnessDefinition("remote", httpConfig)
	require.NoError(t, err)
	for _, tc := range []struct {
		key   string
		value any
	}{
		{"transport", "grpc"}, {"transport", true}, {"protocol", "cli"},
		{"url", "https://agent.example/acp"}, {"url", "ws:///missing-host"}, {"url", "wss://user:password@agent.example/acp"},
		{"url", ""}, {"cwd", ""}, {"cwd", "relative/path"},
		{"binary", "adapter"}, {"prefix_args", []any{}}, {"flag_style", "gnu_long"},
		{"headers", map[string]any{"Authorization": true}},
	} {
		t.Run(tc.key, func(t *testing.T) {
			invalid := make(map[string]any, len(raw))
			maps.Copy(invalid, raw)
			invalid[tc.key] = tc.value
			_, err := parseHarnessDefinition("remote", invalid)
			require.Error(t, err)
		})
	}
	for _, cwd := range []string{`C:\agent\workspace`, `\\agent\workspace`, "${REMOTE_CWD}"} {
		raw["cwd"] = cwd
		_, err := parseHarnessDefinition("remote", raw)
		require.NoError(t, err)
	}
}
