// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"slices"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRebuildDAGFromYAML_PreservesJSONSerializedFields(t *testing.T) {
	t.Parallel()

	// Create a DAG with JSON-serialized fields (typically inherited from base.yaml)
	dag := &core.DAG{
		Name:           "test-dag",
		Queue:          "Default",
		WorkerSelector: map[string]string{"env": "prod"},
		MaxActiveRuns:  5,
		MaxActiveSteps: 3,
		LogDir:         "/custom/logs",
		Tags:           core.NewTags([]string{"important", "production"}),
		Location:       "/path/to/dag.yaml",
		YamlData:       []byte("steps:\n  - name: test\n    command: echo hello"),
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	// Verify JSON-serialized fields are preserved
	assert.Equal(t, "Default", result.Queue)
	assert.Equal(t, map[string]string{"env": "prod"}, result.WorkerSelector)
	assert.Equal(t, 5, result.MaxActiveRuns)
	assert.Equal(t, 3, result.MaxActiveSteps)
	assert.Equal(t, "/custom/logs", result.LogDir)
	assert.Equal(t, []string{"important", "production"}, result.Tags.Strings())
	assert.Equal(t, "/path/to/dag.yaml", result.Location)

	// Verify the original DAG pointer is returned (not a new DAG)
	assert.Same(t, dag, result)
}

func TestRebuildDAGFromYAML_EmptyYAML(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		Queue:    "Default",
		YamlData: nil,
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	assert.Same(t, dag, result)
	assert.Equal(t, "Default", result.Queue)
}

func TestQuoteParamValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     []string
		paramDefs []core.ParamDef
		expect    []string
	}{
		{
			name:   "named param with spaces",
			input:  []string{"topic=hello world"},
			expect: []string{`topic="hello world"`},
		},
		{
			name:   "named param without spaces",
			input:  []string{"topic=hello"},
			expect: []string{`topic="hello"`},
		},
		{
			name:   "positional param with spaces",
			input:  []string{"hello world"},
			expect: []string{`"hello world"`},
		},
		{
			name:   "positional param without spaces",
			input:  []string{"hello"},
			expect: []string{`"hello"`},
		},
		{
			name:   "multiple params",
			input:  []string{"topic=hello world", "count=42", "greeting"},
			expect: []string{`topic="hello world"`, `count="42"`, `"greeting"`},
		},
		{
			name:   "empty slice",
			input:  []string{},
			expect: []string{},
		},
		{
			name:   "param with quotes in value",
			input:  []string{`msg=say "hi"`},
			expect: []string{`msg="say \"hi\""`},
		},
		{
			name:      "positional params stored with numeric placeholders",
			input:     []string{"1=hello world", "2=42"},
			paramDefs: []core.ParamDef{{Name: ""}, {Name: ""}},
			expect:    []string{`"hello world"`, `"42"`},
		},
		{
			name:      "numeric named params stay named",
			input:     []string{"1=hello"},
			paramDefs: []core.ParamDef{{Name: "1"}},
			expect:    []string{`1="hello"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := quoteParamValues(tt.input, tt.paramDefs)
			assert.Equal(t, tt.expect, result)
		})
	}
}

func TestRestoreDAGFromStatus_ParamsWithSpaces(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		YamlData: []byte("params:\n  - topic: \"\"\nsteps:\n  - name: test\n    command: echo $topic"),
	}

	status := &exec.DAGRunStatus{
		ParamsList: []string{"topic=hello world"},
	}

	result, err := restoreDAGFromStatus(context.Background(), dag, status)
	require.NoError(t, err)

	// The restored params should preserve "hello world" as a single value
	found := slices.Contains(result.Params, "topic=hello world")
	assert.True(t, found, "expected 'topic=hello world' in params, got: %v", result.Params)
}

func TestRestoreDAGFromStatus_PositionalParamsRemainOverrides(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		YamlData: []byte("params: \"default\"\nsteps:\n  - name: test\n    command: echo $1"),
		ParamDefs: []core.ParamDef{
			{Name: ""},
		},
	}

	status := &exec.DAGRunStatus{
		ParamsList: []string{"1=override"},
	}

	result, err := restoreDAGFromStatus(context.Background(), dag, status)
	require.NoError(t, err)
	assert.Equal(t, []string{"1=override"}, result.Params)
}

func TestRebuildDAGFromYAML_RebuildEnvFromYAML(t *testing.T) {
	t.Parallel()

	dag := &core.DAG{
		Name:     "test-dag",
		Queue:    "Default",
		Location: "/path/to/dag.yaml",
		YamlData: []byte("env:\n  - MY_VAR: my_value\nsteps:\n  - name: test\n    command: echo $MY_VAR"),
	}

	result, err := rebuildDAGFromYAML(context.Background(), dag)
	require.NoError(t, err)

	assert.Equal(t, "Default", result.Queue)
	assert.Contains(t, result.Env, "MY_VAR=my_value")
}
