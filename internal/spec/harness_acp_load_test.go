// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec_test

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/stretchr/testify/require"
)

// TestACPLoad verifies that base configuration inheritance and cloning preserve ACP.
func TestACPLoad(t *testing.T) {
	base := createTempYAMLFile(t, `harnesses:
  adapter:
    protocol: acp
    transport: websocket
    url: wss://agent.example/acp
    cwd: /remote/workspace
    headers:
      Authorization: Bearer ${ACP_TOKEN}
`)
	child := createTempYAMLFile(t, `steps:
  - action: harness.run
    with:
      provider: adapter
      prompt: hello
`)
	dag, err := spec.Load(context.Background(), child, spec.WithBaseConfig(base))
	require.NoError(t, err)
	require.Equal(t, "acp", dag.Harnesses["adapter"].Protocol)
	require.Equal(t, "adapter", dag.Steps[0].ExecutorConfig.Config["provider"])
	cloned := dag.Clone()
	require.Equal(t, "acp", cloned.Harnesses["adapter"].Protocol)
	require.Equal(t, "websocket", cloned.Harnesses["adapter"].Transport)
	require.Equal(t, "wss://agent.example/acp", cloned.Harnesses["adapter"].URL)
	require.Equal(t, "/remote/workspace", cloned.Harnesses["adapter"].Cwd)
	cloned.Harnesses["adapter"].Headers["Authorization"] = "changed"
	require.Equal(t, "Bearer ${ACP_TOKEN}", dag.Harnesses["adapter"].Headers["Authorization"])
}
