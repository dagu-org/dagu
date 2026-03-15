// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"context"
	"os"
	"path/filepath"
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
  - name: region
    type: string
    enum: [us-east-1, us-west-2]
    required: true
  - name: count
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
  - name: region
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

func TestResolveRuntimeParams_RejectsInvalidEnum(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - name: region
    type: string
    enum: [us-east-1, us-west-2]
    required: true
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	_, err = ResolveRuntimeParams(context.Background(), dag, "region=eu-central-1", ResolveRuntimeParamsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")
}

func TestResolveRuntimeParams_RejectsCoercionError(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - name: count
    type: integer
    required: true
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	_, err = ResolveRuntimeParams(context.Background(), dag, "count=abc", ResolveRuntimeParamsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), `cannot coerce "abc" to integer`)
}

func TestResolveRuntimeParams_RejectsBoundaryViolation(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - name: count
    type: integer
    minimum: 1
    maximum: 5
    required: true
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	_, err = ResolveRuntimeParams(context.Background(), dag, "count=6", ResolveRuntimeParamsOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "count")
}

func TestResolveRuntimeParams_PrefersLocationOverYamlData(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "params.schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(`{
  "type": "object",
  "properties": {
    "region": {
      "type": "string",
      "enum": ["us-east-1", "us-west-2"]
    }
  },
  "required": ["region"]
}`), 0o600))

	dagPath := filepath.Join(dir, "runtime-params.yaml")
	require.NoError(t, os.WriteFile(dagPath, []byte(`
name: runtime-params
params:
  schema: params.schema.json
`), 0o600))

	dag, err := Load(context.Background(), dagPath, WithoutEval())
	require.NoError(t, err)
	require.NotEmpty(t, dag.Location)
	require.NotEmpty(t, dag.YamlData)

	resolved, err := ResolveRuntimeParams(context.Background(), dag, "region=us-east-1", ResolveRuntimeParamsOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"region=us-east-1"}, resolved.Params)
}

func TestResolveRuntimeParams_LegacyNamedParamsPreservePositionalOverrides(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-params
params:
  - TAG: ""
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	resolved, err := ResolveRuntimeParams(context.Background(), dag, []string{"simple"}, ResolveRuntimeParamsOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{"TAG=", "1=simple"}, resolved.Params)
}

func TestResolveRuntimeParams_EvaluatesParamEvalOnReload(t *testing.T) {
	t.Setenv("WORK_DIR", "/tmp/work")

	yaml := []byte(`
name: runtime-eval
params:
  - name: base_dir
    eval: "$WORK_DIR/pipeline"
  - name: output_dir
    eval: "$base_dir/output"
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	resolved, err := ResolveRuntimeParams(context.Background(), dag, "", ResolveRuntimeParamsOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"base_dir=/tmp/work/pipeline",
		"output_dir=/tmp/work/pipeline/output",
	}, resolved.Params)
	assert.Equal(t, `base_dir="/tmp/work/pipeline" output_dir="/tmp/work/pipeline/output"`, resolved.DefaultParams)
}

func TestResolveRuntimeParams_OverrideBeatsEvalAndFeedsLaterParams(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: runtime-eval-override
params:
  - name: base_dir
    eval: "/default/base"
  - name: output_dir
    eval: "$base_dir/output"
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	dag.YamlData = yaml

	resolved, err := ResolveRuntimeParams(context.Background(), dag, "base_dir=/custom/base", ResolveRuntimeParamsOptions{})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"base_dir=/custom/base",
		"output_dir=/custom/base/output",
	}, resolved.Params)
	assert.JSONEq(t, `{"base_dir":"/custom/base","output_dir":"/custom/base/output"}`, resolved.ParamsJSON)
}

func TestToFloat64_RejectsUnsafeIntegerPrecision(t *testing.T) {
	t.Parallel()

	_, err := toFloat64(int64(maxSafeFloat64Integer + 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "safe range")
}

func TestInt64FromUint64_ConvertsDirectly(t *testing.T) {
	t.Parallel()

	value, err := int64FromUint64(maxInt64AsUint)
	require.NoError(t, err)
	assert.Equal(t, int64(maxInt64AsUint), value)
}
