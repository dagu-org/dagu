package agent

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestThinkTool_Run(t *testing.T) {
	t.Parallel()

	tool := NewThinkTool()

	tests := []struct {
		name         string
		input        string
		wantContains string
	}{
		{
			name:         "returns acknowledgment",
			input:        `{"thought": "Let me think about this..."}`,
			wantContains: "Thought recorded",
		},
		{
			name:  "works with empty thought",
			input: `{"thought": ""}`,
		},
		{
			name:  "works with empty input",
			input: `{}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := tool.Run(ToolContext{}, json.RawMessage(tc.input))

			assert.False(t, result.IsError)
			if tc.wantContains != "" {
				assert.Contains(t, result.Content, tc.wantContains)
			}
		})
	}
}

func TestNewThinkTool(t *testing.T) {
	t.Parallel()

	tool := NewThinkTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "think", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Run)
}
