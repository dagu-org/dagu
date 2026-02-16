package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDelegateTool_NoDelegateContext(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{}, json.RawMessage(`{"task": "test"}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "not available")
}

func TestDelegateTool_EmptyTask(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`{"task": ""}`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "required")
}

func TestDelegateTool_InvalidInput(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()
	result := tool.Run(ToolContext{
		Context:  context.Background(),
		Delegate: &DelegateContext{},
	}, json.RawMessage(`invalid json`))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "Invalid input")
}

func TestDelegateTool_Schema(t *testing.T) {
	t.Parallel()

	tool := NewDelegateTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "delegate", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Function.Parameters)

	params := tool.Function.Parameters
	require.NotNil(t, params)
	assert.Equal(t, "object", params["type"])

	props, ok := params["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "task")
	assert.Contains(t, props, "max_iterations")

	required, ok := params["required"].([]string)
	require.True(t, ok)
	assert.Contains(t, required, "task")
}

func TestFilterOutTool(t *testing.T) {
	t.Parallel()

	tools := []*AgentTool{
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "bash"}}},
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "delegate"}}},
		{Tool: llm.Tool{Function: llm.ToolFunction{Name: "read"}}},
	}

	t.Run("removes named tool", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "delegate")
		assert.Len(t, filtered, 2)
		for _, tool := range filtered {
			assert.NotEqual(t, "delegate", tool.Function.Name)
		}
	})

	t.Run("preserves order", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "delegate")
		require.Len(t, filtered, 2)
		assert.Equal(t, "bash", filtered[0].Function.Name)
		assert.Equal(t, "read", filtered[1].Function.Name)
	})

	t.Run("no-op for unknown name", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(tools, "unknown")
		assert.Len(t, filtered, 3)
	})

	t.Run("handles empty slice", func(t *testing.T) {
		t.Parallel()
		filtered := filterOutTool(nil, "delegate")
		assert.Empty(t, filtered)
	})
}

func TestTruncate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string unchanged", "hello", 10, "hello"},
		{"exact length unchanged", "hello", 5, "hello"},
		{"long string truncated", "hello world", 5, "hello..."},
		{"empty string", "", 5, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, truncate(tc.input, tc.maxLen))
		})
	}
}
