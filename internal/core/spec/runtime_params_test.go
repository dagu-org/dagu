// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveRuntimeParams(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - region:
      type: string
      enum: [us-east-1, us-west-2]
      required: true
  - count:
      default: 3
      type: integer
      minimum: 1
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	resolved, err := ResolveRuntimeParams(context.Background(), dag, "region=us-west-2 count=5", ResolveRuntimeParamsOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"region=us-west-2", "count=5"}, resolved.Params)
	assert.JSONEq(t, `{"region":"us-west-2","count":"5"}`, resolved.ParamsJSON)
}

func TestResolveRuntimeParams_ValidatesEmptyRuntimeInput(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - region:
      type: string
      required: true
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	_, err = ResolveRuntimeParams(context.Background(), dag, "", ResolveRuntimeParamsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")
}

func TestResolveRuntimeParams_RequiresSource(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{Name: "runtime-params"}

	_, err := ResolveRuntimeParams(context.Background(), dag, "", ResolveRuntimeParamsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "source")
}
