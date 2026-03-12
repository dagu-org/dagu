// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/goccy/go-yaml"

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
		name   string
		input  []string
		expect []string
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := quoteParamValues(tt.input)
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

// --- syncYAMLData tests ---

// unmarshalYAMLMap is a test helper that parses YAML into an ordered map.
func unmarshalYAMLMap(t *testing.T, data []byte) yaml.MapSlice {
	t.Helper()
	var ms yaml.MapSlice
	require.NoError(t, yaml.Unmarshal(data, &ms))
	return ms
}

func TestSyncYAMLData(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		dag     *core.DAG
		nilYAML bool
		checkFn func(t *testing.T, dag *core.DAG, err error)
	}{
		{
			name: "no overrides - tags match YAML",
			yaml: "name: test\ntags:\n- env=prod\nsteps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"env=prod"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				// Fast path: original bytes preserved
				assert.Equal(t, "name: test\ntags:\n- env=prod\nsteps:\n- name: s1\n  command: echo hi\n", string(dag.YamlData))
			},
		},
		{
			name: "tags appended - YAML has no tags",
			yaml: "name: test\nsteps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"workspace=ops"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, ok := getMapSliceValue(ms, "tags")
				require.True(t, ok, "tags key should exist")
				tags, ok := v.([]any)
				require.True(t, ok)
				assert.Equal(t, []any{"workspace=ops"}, tags)
			},
		},
		{
			name: "tags replaced - YAML has different tags",
			yaml: "name: test\ntags:\n- old=tag\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"new=tag", "workspace=ops"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, _ := getMapSliceValue(ms, "tags")
				tags, ok := v.([]any)
				require.True(t, ok)
				assert.Len(t, tags, 2)
				assert.Contains(t, tags, "new=tag")
				assert.Contains(t, tags, "workspace=ops")
			},
		},
		{
			name: "tags in map format with runtime append",
			yaml: "name: test\ntags:\n  foo: bar\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"foo=bar", "workspace=ops"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, _ := getMapSliceValue(ms, "tags")
				tags, ok := v.([]any)
				require.True(t, ok, "tags should be array-of-strings after patching")
				assert.Len(t, tags, 2)
			},
		},
		{
			name: "multi-document YAML - only first doc patched",
			yaml: "name: main\nsteps:\n- name: s1\n  command: echo hi\n---\nname: sub\ntags:\n- keep=me\nsteps:\n- name: s2\n  command: echo bye\n",
			dag: &core.DAG{
				Name: "main",
				Tags: core.NewTags([]string{"added=tag"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				content := string(dag.YamlData)
				assert.Contains(t, content, "added=tag", "first doc should have new tag")
				assert.Contains(t, content, "---", "document separator preserved")
				assert.Contains(t, content, "keep=me", "second doc preserved")
			},
		},
		{
			name: "--- inside block scalar is not a document separator",
			yaml: "name: test\nsteps:\n- name: s1\n  command: |\n    echo start\n    echo ---\n    echo end\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"runtime=tag"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, ok := getMapSliceValue(ms, "tags")
				require.True(t, ok)
				// The block scalar command should survive intact
				stepsRaw, ok := getMapSliceValue(ms, "steps")
				require.True(t, ok)
				steps, ok := stepsRaw.([]any)
				require.True(t, ok)
				require.Len(t, steps, 1)
				_ = v // tags exist, that's the main assertion
			},
		},
		{
			name: "empty YamlData",
			yaml: "",
			dag: &core.DAG{
				Tags: core.NewTags([]string{"a=b"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				assert.Empty(t, dag.YamlData)
			},
		},
		{
			name:    "nil YamlData",
			nilYAML: true,
			dag: &core.DAG{
				Tags: core.NewTags([]string{"a=b"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				assert.Nil(t, dag.YamlData)
			},
		},
		{
			name: "invalid YAML returns error, data unchanged",
			yaml: "{{invalid yaml",
			dag: &core.DAG{
				Tags: core.NewTags([]string{"a=b"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				assert.Error(t, err)
				assert.Equal(t, "{{invalid yaml", string(dag.YamlData))
			},
		},
		{
			name: "queue override - YAML has no queue",
			yaml: "name: test\nsteps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name:  "test",
				Queue: "priority",
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, ok := getMapSliceString(ms, "queue")
				require.True(t, ok)
				assert.Equal(t, "priority", v)
			},
		},
		{
			name: "queue override - YAML has different queue",
			yaml: "name: test\nqueue: default\n",
			dag: &core.DAG{
				Name:  "test",
				Queue: "fast",
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, _ := getMapSliceString(ms, "queue")
				assert.Equal(t, "fast", v)
			},
		},
		{
			name: "name override - YAML has name",
			yaml: "name: original\nsteps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name: "overridden",
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, _ := getMapSliceString(ms, "name")
				assert.Equal(t, "overridden", v)
			},
		},
		{
			name: "name NOT injected when YAML has no name key",
			yaml: "steps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name: "auto-derived",
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				// Original bytes preserved (no name key to trigger change, no other diffs)
				assert.Equal(t, "steps:\n- name: s1\n  command: echo hi\n", string(dag.YamlData))
			},
		},
		{
			name: "nil tags in DAG struct removes tags from YAML",
			yaml: "name: test\ntags:\n- old=tag\n",
			dag: &core.DAG{
				Name: "test",
				Tags: nil,
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				_, hasTags := getMapSliceValue(ms, "tags")
				assert.False(t, hasTags, "tags key should be removed")
			},
		},
		{
			name: "combined overrides - tags + queue + name",
			yaml: "name: orig\nqueue: default\ntags:\n- old=tag\nsteps:\n- name: s1\n  command: echo hi\n",
			dag: &core.DAG{
				Name:  "newname",
				Queue: "fast",
				Tags:  core.NewTags([]string{"env=prod", "workspace=ops"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				name, _ := getMapSliceString(ms, "name")
				assert.Equal(t, "newname", name)
				queue, _ := getMapSliceString(ms, "queue")
				assert.Equal(t, "fast", queue)
				v, _ := getMapSliceValue(ms, "tags")
				tags, ok := v.([]any)
				require.True(t, ok)
				assert.Len(t, tags, 2)
				// Verify key ordering is preserved (name, queue, tags, steps)
				assert.Equal(t, "name", fmt.Sprint(ms[0].Key))
				assert.Equal(t, "queue", fmt.Sprint(ms[1].Key))
				assert.Equal(t, "tags", fmt.Sprint(ms[2].Key))
				assert.Equal(t, "steps", fmt.Sprint(ms[3].Key))
			},
		},
		{
			name: "tags with special chars preserved",
			yaml: "name: test\n",
			dag: &core.DAG{
				Name: "test",
				Tags: core.NewTags([]string{"workspace=ops/team-a", "env=prod.v2"}),
			},
			checkFn: func(t *testing.T, dag *core.DAG, err error) {
				require.NoError(t, err)
				ms := unmarshalYAMLMap(t, dag.YamlData)
				v, _ := getMapSliceValue(ms, "tags")
				tags, ok := v.([]any)
				require.True(t, ok)
				assert.Contains(t, tags, "workspace=ops/team-a")
				assert.Contains(t, tags, "env=prod.v2")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag := tt.dag
			if tt.nilYAML {
				dag.YamlData = nil
			} else {
				dag.YamlData = []byte(tt.yaml)
			}
			err := syncYAMLData(dag)
			tt.checkFn(t, dag, err)
		})
	}
}

func TestExtractTagStringsFromMapSlice(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		yaml   string
		expect []string
	}{
		{
			name:   "array of strings",
			yaml:   "tags:\n- foo=bar\n- env=prod\n",
			expect: []string{"foo=bar", "env=prod"},
		},
		{
			name:   "map format",
			yaml:   "tags:\n  foo: bar\n  env: prod\n",
			expect: []string{"env=prod", "foo=bar"}, // sorted for stable comparison
		},
		{
			name:   "space-separated string",
			yaml:   "tags: \"foo=bar env=prod\"\n",
			expect: []string{"foo=bar", "env=prod"},
		},
		{
			name:   "no tags key",
			yaml:   "name: test\n",
			expect: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var ms yaml.MapSlice
			require.NoError(t, yaml.Unmarshal([]byte(tt.yaml), &ms))
			result := extractTagStringsFromMapSlice(ms)
			if tt.expect == nil {
				assert.Nil(t, result)
			} else {
				assert.ElementsMatch(t, tt.expect, result)
			}
		})
	}
}

func TestMapSliceHelpers(t *testing.T) {
	t.Parallel()

	t.Run("get existing key", func(t *testing.T) {
		t.Parallel()
		ms := yaml.MapSlice{{Key: "name", Value: "test"}, {Key: "queue", Value: "fast"}}
		v, ok := getMapSliceValue(ms, "queue")
		assert.True(t, ok)
		assert.Equal(t, "fast", v)
	})

	t.Run("get missing key", func(t *testing.T) {
		t.Parallel()
		ms := yaml.MapSlice{{Key: "name", Value: "test"}}
		_, ok := getMapSliceValue(ms, "missing")
		assert.False(t, ok)
	})

	t.Run("set existing key preserves order", func(t *testing.T) {
		t.Parallel()
		ms := yaml.MapSlice{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}, {Key: "c", Value: "3"}}
		setMapSliceValue(&ms, "b", "updated")
		assert.Equal(t, "a", fmt.Sprint(ms[0].Key))
		assert.Equal(t, "updated", ms[1].Value)
		assert.Equal(t, "c", fmt.Sprint(ms[2].Key))
		assert.Len(t, ms, 3)
	})

	t.Run("set new key appends", func(t *testing.T) {
		t.Parallel()
		ms := yaml.MapSlice{{Key: "a", Value: "1"}}
		setMapSliceValue(&ms, "b", "2")
		assert.Len(t, ms, 2)
		assert.Equal(t, "b", fmt.Sprint(ms[1].Key))
		assert.Equal(t, "2", ms[1].Value)
	})

	t.Run("remove existing key", func(t *testing.T) {
		t.Parallel()
		ms := yaml.MapSlice{{Key: "a", Value: "1"}, {Key: "b", Value: "2"}, {Key: "c", Value: "3"}}
		removeMapSliceKey(&ms, "b")
		assert.Len(t, ms, 2)
		assert.Equal(t, "a", fmt.Sprint(ms[0].Key))
		assert.Equal(t, "c", fmt.Sprint(ms[1].Key))
	})
}
