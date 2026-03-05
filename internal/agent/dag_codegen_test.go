package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGCodegenRun(t *testing.T) {
	dagsDir := t.TempDir()

	tool := NewDAGCodegenTool(dagsDir)
	require.NotNil(t, tool)

	t.Run("valid_parallel_steps", func(t *testing.T) {
		input := DAGCodegenInput{
			Name: "test-parallel",
			Steps: []DAGCodegenStep{
				{Name: "step1", Command: "echo", Args: []string{"hello"}},
				{Name: "step2", Command: "echo", Args: []string{"world"}},
			},
			Tags: []string{"workspace:test"},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.False(t, out.IsError, "unexpected error: %s", out.Content)
		assert.Contains(t, out.Content, "Created DAG 'test-parallel'")
		assert.Contains(t, out.Content, "2 steps")

		filePath := filepath.Join(dagsDir, ".generated", "test-parallel.yaml")
		assert.FileExists(t, filePath)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Contains(t, string(content), "workspace:test")
	})

	t.Run("valid_sequential_steps", func(t *testing.T) {
		input := DAGCodegenInput{
			Name: "test-sequential",
			Steps: []DAGCodegenStep{
				{Name: "build", Command: "make", Args: []string{"build"}},
				{Name: "test", Command: "make", Args: []string{"test"}, Depends: []string{"build"}},
			},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.False(t, out.IsError, "unexpected error: %s", out.Content)

		content, err := os.ReadFile(filepath.Join(dagsDir, ".generated", "test-sequential.yaml"))
		require.NoError(t, err)
		assert.Contains(t, string(content), "depends:")
		assert.Contains(t, string(content), "- build")
	})

	t.Run("duplicate_step_name", func(t *testing.T) {
		input := DAGCodegenInput{
			Name: "test-dup",
			Steps: []DAGCodegenStep{
				{Name: "step1", Command: "echo"},
				{Name: "step1", Command: "echo"},
			},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "duplicate step name")
	})

	t.Run("cycle_detection", func(t *testing.T) {
		input := DAGCodegenInput{
			Name: "test-cycle",
			Steps: []DAGCodegenStep{
				{Name: "a", Command: "echo", Depends: []string{"b"}},
				{Name: "b", Command: "echo", Depends: []string{"a"}},
			},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "cycle")
	})

	t.Run("unknown_dependency", func(t *testing.T) {
		input := DAGCodegenInput{
			Name: "test-unknown-dep",
			Steps: []DAGCodegenStep{
				{Name: "step1", Command: "echo", Depends: []string{"nonexistent"}},
			},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "unknown step")
	})

	t.Run("empty_name", func(t *testing.T) {
		input := DAGCodegenInput{
			Steps: []DAGCodegenStep{
				{Name: "step1", Command: "echo"},
			},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "name is required")
	})

	t.Run("no_steps", func(t *testing.T) {
		input := DAGCodegenInput{
			Name:  "test-empty",
			Steps: []DAGCodegenStep{},
		}
		raw, err := json.Marshal(input)
		require.NoError(t, err)

		out := tool.Run(ToolContext{Context: context.Background()}, raw)
		assert.True(t, out.IsError)
		assert.Contains(t, out.Content, "at least one step")
	})
}

func TestValidateSteps(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		steps := []DAGCodegenStep{
			{Name: "a", Command: "echo"},
			{Name: "b", Command: "echo", Depends: []string{"a"}},
			{Name: "c", Command: "echo", Depends: []string{"a", "b"}},
		}
		assert.NoError(t, validateSteps(steps))
	})

	t.Run("empty_step_name", func(t *testing.T) {
		steps := []DAGCodegenStep{
			{Name: "", Command: "echo"},
		}
		assert.Error(t, validateSteps(steps))
	})
}

func TestDetectCycle(t *testing.T) {
	t.Run("no_cycle", func(t *testing.T) {
		steps := []DAGCodegenStep{
			{Name: "a", Command: "echo"},
			{Name: "b", Command: "echo", Depends: []string{"a"}},
		}
		assert.NoError(t, detectCycle(steps))
	})

	t.Run("self_cycle", func(t *testing.T) {
		steps := []DAGCodegenStep{
			{Name: "a", Command: "echo", Depends: []string{"a"}},
		}
		assert.Error(t, detectCycle(steps))
	})

	t.Run("indirect_cycle", func(t *testing.T) {
		steps := []DAGCodegenStep{
			{Name: "a", Command: "echo", Depends: []string{"c"}},
			{Name: "b", Command: "echo", Depends: []string{"a"}},
			{Name: "c", Command: "echo", Depends: []string{"b"}},
		}
		assert.Error(t, detectCycle(steps))
	})
}

func TestBuildDAGYAML(t *testing.T) {
	input := DAGCodegenInput{
		Name: "my-dag",
		Steps: []DAGCodegenStep{
			{Name: "step1", Command: "echo", Args: []string{"hello world"}, Dir: "/tmp"},
			{Name: "step2", Command: "ls", Depends: []string{"step1"}},
		},
		Tags: []string{"workspace:test", "env:dev"},
	}

	yaml := buildDAGYAML(input)

	assert.Contains(t, yaml, "name: my-dag")
	assert.Contains(t, yaml, "type: graph")
	assert.Contains(t, yaml, "- workspace:test")
	assert.Contains(t, yaml, "- env:dev")
	assert.Contains(t, yaml, "name: step1")
	assert.Contains(t, yaml, "command: echo")
	assert.Contains(t, yaml, "dir: /tmp")
	assert.Contains(t, yaml, "name: step2")
	assert.Contains(t, yaml, "- step1")
}
