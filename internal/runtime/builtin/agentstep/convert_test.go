package agentstep

import (
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testModelConfig() *agent.ModelConfig {
	return &agent.ModelConfig{
		Provider: "openai",
		Model:    "gpt-4",
	}
}

func TestConvertMessage_AssistantWithUsageAndCost(t *testing.T) {
	t.Parallel()

	cost := 0.0042
	msg := agent.Message{
		Type:    agent.MessageTypeAssistant,
		Content: "hello world",
		Usage: &llm.Usage{
			PromptTokens:     100,
			CompletionTokens: 50,
			TotalTokens:      150,
		},
		Cost: &cost,
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 1)
	m := result[0]
	assert.Equal(t, exec.RoleAssistant, m.Role)
	assert.Equal(t, "hello world", m.Content)
	require.NotNil(t, m.Metadata)
	assert.Equal(t, "openai", m.Metadata.Provider)
	assert.Equal(t, "gpt-4", m.Metadata.Model)
	assert.Equal(t, 100, m.Metadata.PromptTokens)
	assert.Equal(t, 50, m.Metadata.CompletionTokens)
	assert.Equal(t, 150, m.Metadata.TotalTokens)
	assert.InDelta(t, 0.0042, m.Metadata.Cost, 1e-9)
}

func TestConvertMessage_AssistantWithToolCalls(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type:    agent.MessageTypeAssistant,
		Content: "",
		ToolCalls: []llm.ToolCall{
			{
				ID:   "call_1",
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      "bash",
					Arguments: `{"command":"ls"}`,
				},
			},
			{
				ID:   "call_2",
				Type: "function",
				Function: llm.ToolCallFunction{
					Name:      "read",
					Arguments: `{"path":"/tmp/test"}`,
				},
			},
		},
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 1)
	m := result[0]
	assert.Equal(t, exec.RoleAssistant, m.Role)
	require.Len(t, m.ToolCalls, 2)

	assert.Equal(t, "call_1", m.ToolCalls[0].ID)
	assert.Equal(t, "function", m.ToolCalls[0].Type)
	assert.Equal(t, "bash", m.ToolCalls[0].Function.Name)
	assert.Equal(t, `{"command":"ls"}`, m.ToolCalls[0].Function.Arguments)

	assert.Equal(t, "call_2", m.ToolCalls[1].ID)
	assert.Equal(t, "read", m.ToolCalls[1].Function.Name)
}

func TestConvertMessage_UserNoToolResults(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type:    agent.MessageTypeUser,
		Content: "user input",
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 1)
	assert.Equal(t, exec.RoleUser, result[0].Role)
	assert.Equal(t, "user input", result[0].Content)
	assert.Nil(t, result[0].Metadata)
}

func TestConvertMessage_UserWithToolResults(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type: agent.MessageTypeUser,
		ToolResults: []agent.ToolResult{
			{ToolCallID: "call_1", Content: "result 1"},
			{ToolCallID: "call_2", Content: "result 2"},
		},
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 2)

	assert.Equal(t, exec.RoleTool, result[0].Role)
	assert.Equal(t, "result 1", result[0].Content)
	assert.Equal(t, "call_1", result[0].ToolCallID)

	assert.Equal(t, exec.RoleTool, result[1].Role)
	assert.Equal(t, "result 2", result[1].Content)
	assert.Equal(t, "call_2", result[1].ToolCallID)
}

func TestConvertMessage_NilUsageAndCost(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type:    agent.MessageTypeAssistant,
		Content: "response",
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 1)
	m := result[0]
	require.NotNil(t, m.Metadata)
	assert.Equal(t, "openai", m.Metadata.Provider)
	assert.Equal(t, "gpt-4", m.Metadata.Model)
	assert.Equal(t, 0, m.Metadata.PromptTokens)
	assert.Equal(t, 0, m.Metadata.CompletionTokens)
	assert.Equal(t, 0, m.Metadata.TotalTokens)
	assert.InDelta(t, 0.0, m.Metadata.Cost, 1e-9)
}

func TestConvertMessage_ErrorType(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type:    agent.MessageTypeError,
		Content: "something went wrong",
	}

	result := convertMessage(msg, testModelConfig())

	require.Len(t, result, 1)
	assert.Equal(t, exec.RoleAssistant, result[0].Role)
	assert.Equal(t, "something went wrong", result[0].Content)
}

func TestConvertMessage_UnknownType(t *testing.T) {
	t.Parallel()

	msg := agent.Message{
		Type:    agent.MessageTypeUIAction,
		Content: "navigate",
	}

	result := convertMessage(msg, testModelConfig())
	assert.Nil(t, result)
}
