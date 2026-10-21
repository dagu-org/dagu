// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package schema

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDAGSchemaACP keeps schema protocol validation aligned with the YAML parser.
func TestDAGSchemaACP(t *testing.T) {
	const source = `harnesses:
  adapter:
    binary: adapter
    protocol: acp
steps:
  - action: harness.run
    with:
      provider: adapter
      prompt: hello
`
	resolved := mustResolveDAGSchema(t)
	require.NoError(t, resolved.Validate(mustParseYAMLDocument(t, source)))
	for _, replacement := range []string{"protocol: http", "protocol: true", "protocol: acp\n    prompt_mode: arg", "protocol: acp\n    prompt_flag: -p", "protocol: acp\n    prompt_position: after_flags"} {
		require.Error(t, resolved.Validate(mustParseYAMLDocument(t, strings.Replace(source, "protocol: acp", replacement, 1))))
	}
}
