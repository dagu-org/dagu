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

func TestInlineParamDefs_MetadataAndExecution(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: inline-params
params:
  - region:
      description: AWS region
      type: string
      enum: [us-east-1, us-west-2]
      required: true
  - instance_count:
      default: 3
      type: integer
      minimum: 1
      maximum: 10
  - debug:
      default: false
      type: boolean
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	require.Len(t, dag.ParamDefs, 3)
	assert.Equal(t, "region", dag.ParamDefs[0].Name)
	assert.Equal(t, core.ParamDefTypeString, dag.ParamDefs[0].Type)
	assert.True(t, dag.ParamDefs[0].Required)
	assert.Nil(t, dag.ParamDefs[0].Default)
	assert.Equal(t, int64(3), dag.ParamDefs[1].Default)
	assert.Equal(t, false, dag.ParamDefs[2].Default)
	assert.Equal(t, `instance_count="3" debug="false"`, dag.DefaultParams)
	assert.JSONEq(t, `{"instance_count":"3","debug":"false"}`, dag.ParamsJSON)

	_, err = LoadYAML(context.Background(), yaml, WithoutEval(), WithParams(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")

	dag, err = LoadYAML(
		context.Background(),
		yaml,
		WithoutEval(),
		WithParams(`[{"region":"us-west-2"},{"instance_count":"5"},{"debug":"true"}]`),
	)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"region=us-west-2",
		"instance_count=5",
		"debug=true",
	}, dag.Params)
	assert.JSONEq(t, `[{"region":"us-west-2"},{"instance_count":"5"},{"debug":"true"}]`, dag.ParamsJSON)
}

func TestInlineParamDefs_MixedLegacyAndInline(t *testing.T) {
	t.Parallel()

	yaml := []byte(`
name: mixed-params
params:
  - environment:
      type: string
      enum: [dev, staging, prod]
      default: staging
  - TAG: latest
  - DRY_RUN: "true"
`)

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	require.Len(t, dag.ParamDefs, 3)

	assert.Equal(t, "environment", dag.ParamDefs[0].Name)
	assert.Equal(t, core.ParamDefTypeString, dag.ParamDefs[0].Type)
	assert.Equal(t, "staging", dag.ParamDefs[0].Default)

	assert.Equal(t, "TAG", dag.ParamDefs[1].Name)
	assert.Equal(t, core.ParamDefTypeString, dag.ParamDefs[1].Type)
	assert.Equal(t, "latest", dag.ParamDefs[1].Default)

	assert.Equal(t, "DRY_RUN", dag.ParamDefs[2].Name)
	assert.Equal(t, "true", dag.ParamDefs[2].Default)
	assert.Equal(t, `environment="staging" TAG="latest" DRY_RUN="true"`, dag.DefaultParams)
}

func TestExternalSchemaParamDefs_MetadataAndExecution(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	schemaPath := filepath.Join(dir, "params.schema.json")
	require.NoError(t, os.WriteFile(schemaPath, []byte(`{
  "type": "object",
  "properties": {
    "region": {
      "type": "string",
      "description": "Target region",
      "enum": ["us-east-1", "us-west-2"]
    },
    "count": {
      "type": "integer",
      "minimum": 1,
      "maximum": 10,
      "default": 3
    }
  },
  "required": ["region"]
}`), 0o600))

	yaml := []byte("name: schema-params\nparams:\n  schema: " + schemaPath + "\n")

	dag, err := LoadYAML(context.Background(), yaml, WithoutEval())
	require.NoError(t, err)
	require.Len(t, dag.ParamDefs, 2)
	assert.Equal(t, "count", dag.ParamDefs[0].Name)
	assert.Equal(t, float64(3), dag.ParamDefs[0].Default)
	assert.Equal(t, "region", dag.ParamDefs[1].Name)
	assert.True(t, dag.ParamDefs[1].Required)
	assert.Nil(t, dag.ParamDefs[1].Default)
	assert.Equal(t, `count="3"`, dag.DefaultParams)

	_, err = LoadYAML(context.Background(), yaml, WithoutEval(), WithParams(""))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "region")

	dag, err = LoadYAML(context.Background(), yaml, WithoutEval(), WithParams("region=us-east-1 count=4"))
	require.NoError(t, err)
	assert.Equal(t, []string{"count=4", "region=us-east-1"}, dag.Params)
	assert.JSONEq(t, `{"region":"us-east-1","count":"4"}`, dag.ParamsJSON)
}
