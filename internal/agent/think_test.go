package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThinkTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("returns acknowledgment", func(t *testing.T) {
		t.Parallel()

		tool := NewThinkTool()
		input := json.RawMessage(`{"thought": "Let me think about this..."}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Thought recorded")
	})

	t.Run("works with empty thought", func(t *testing.T) {
		t.Parallel()

		tool := NewThinkTool()
		input := json.RawMessage(`{"thought": ""}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
	})

	t.Run("works with empty input", func(t *testing.T) {
		t.Parallel()

		tool := NewThinkTool()
		input := json.RawMessage(`{}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
	})
}

func TestNewThinkTool(t *testing.T) {
	t.Parallel()

	tool := NewThinkTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "think", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Run)
}
