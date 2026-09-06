// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec_test

import (
	"context"
	"testing"

	"github.com/dagucloud/dagu/v2/internal/spec"
	"github.com/stretchr/testify/require"
)

// Regression tests for patchOrderedEnvValues rewriting any mapping keyed
// "env" into the DAG's own ordered env-list shape, even when that mapping
// belongs to unrelated data nested under a step's with:/params: field (see
// internal/spec/manifest_decoder.go's opaqueDataKeys).

func TestPreserveEnvMappingOrder_ParamsSchemaEnvPropertyDoesNotBreakDecode(t *testing.T) {
	t.Parallel()

	_, err := spec.LoadYAML(context.Background(), []byte(`
name: params-schema-env-property
params:
  type: object
  properties:
    env:
      type: string
      default: ""
steps:
  - name: run
    run: echo hi
`))
	require.NoError(t, err)
}

func TestPreserveEnvMappingOrder_WithEnvStaysPlainObject(t *testing.T) {
	t.Parallel()

	dag, err := spec.LoadYAML(context.Background(), []byte(`
name: with-env-plain-object
steps:
  - name: run
    action: node-script@v1
    with:
      script: return 1
      env:
        FOO: bar
`))
	require.NoError(t, err)
	input, ok := dag.Steps[0].ExecutorConfig.Config["input"].(map[string]any)
	require.True(t, ok, "action executor config should nest with: under input")
	require.Equal(t, map[string]any{"FOO": "bar"}, input["env"])
}

func TestPreserveEnvMappingOrder_RootEnvOrderStillPreserved(t *testing.T) {
	t.Parallel()

	dag, err := spec.LoadYAML(context.Background(), []byte(`
name: root-env-order
env:
  FIRST: one
  SECOND: ${env.FIRST}-two
steps:
  - name: run
    run: echo hi
`))
	require.NoError(t, err)
	require.Equal(t, []string{"FIRST=one", "SECOND=one-two"}, dag.Env)
}
