// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec_test

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/stretchr/testify/require"
)

func TestACPLoad(t *testing.T) {
	base := createTempYAMLFile(t, `harnesses:
  adapter:
    binary: adapter
    protocol: acp
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
}
