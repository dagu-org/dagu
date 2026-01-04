package chat

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	llmpkg "github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutor_MessageSaving(t *testing.T) {
	t.Parallel()

	t.Run("SavesAllMessagesWithInherited", func(t *testing.T) {
		t.Parallel()

		executor := &Executor{
			step: core.Step{
				LLM: &core.LLMConfig{
					Provider: "openai",
					Model:    "gpt-4o",
				},
			},
			messages: []execution.LLMMessage{
				{Role: execution.RoleUser, Content: "Hello"},
			},
			contextMessages: []execution.LLMMessage{
				{Role: execution.RoleSystem, Content: "You are helpful"},
				{Role: execution.RoleUser, Content: "Previous question"},
				{Role: execution.RoleAssistant, Content: "Previous answer"},
			},
		}

		// Simulate what happens after Run() completes
		allMessages := append(executor.contextMessages, executor.messages...)
		metadata := &execution.LLMMessageMetadata{
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		}
		executor.savedMessages = append(allMessages, execution.LLMMessage{
			Role:     execution.RoleAssistant,
			Content:  "Hello there!",
			Metadata: metadata,
		})

		saved := executor.GetMessages()
		assert.Len(t, saved, 5) // 3 inherited + 1 user + 1 assistant
		assert.Equal(t, execution.RoleSystem, saved[0].Role)
		assert.Equal(t, execution.RoleAssistant, saved[4].Role)
		assert.NotNil(t, saved[4].Metadata)
		assert.Equal(t, 15, saved[4].Metadata.TotalTokens)
	})

	t.Run("MetadataAlwaysSaved", func(t *testing.T) {
		t.Parallel()

		executor := &Executor{
			step: core.Step{
				LLM: &core.LLMConfig{
					Provider: "gemini",
					Model:    "gemini-pro",
				},
			},
			messages: []execution.LLMMessage{
				{Role: execution.RoleUser, Content: "Test"},
			},
		}

		metadata := &execution.LLMMessageMetadata{
			Provider:         "gemini",
			Model:            "gemini-pro",
			PromptTokens:     5,
			CompletionTokens: 3,
			TotalTokens:      8,
		}
		executor.savedMessages = append(executor.messages, execution.LLMMessage{
			Role:     execution.RoleAssistant,
			Content:  "Response",
			Metadata: metadata,
		})

		saved := executor.GetMessages()
		assert.Len(t, saved, 2)

		assistantMsg := saved[1]
		assert.Equal(t, execution.RoleAssistant, assistantMsg.Role)
		assert.NotNil(t, assistantMsg.Metadata)
		assert.Equal(t, "gemini", assistantMsg.Metadata.Provider)
		assert.Equal(t, "gemini-pro", assistantMsg.Metadata.Model)
		assert.Equal(t, 5, assistantMsg.Metadata.PromptTokens)
		assert.Equal(t, 3, assistantMsg.Metadata.CompletionTokens)
		assert.Equal(t, 8, assistantMsg.Metadata.TotalTokens)
	})
}

func TestExecutor_SetContext(t *testing.T) {
	t.Parallel()

	executor := &Executor{}

	messages := []execution.LLMMessage{
		{Role: execution.RoleSystem, Content: "System prompt"},
		{Role: execution.RoleUser, Content: "User message"},
		{Role: execution.RoleAssistant, Content: "Assistant response"},
	}

	executor.SetContext(messages)

	assert.Equal(t, messages, executor.contextMessages)
}

func TestExecutor_GetMessages(t *testing.T) {
	t.Parallel()

	t.Run("ReturnsNilWhenEmpty", func(t *testing.T) {
		t.Parallel()

		executor := &Executor{}
		assert.Nil(t, executor.GetMessages())
	})

	t.Run("ReturnsSavedMessages", func(t *testing.T) {
		t.Parallel()

		executor := &Executor{
			savedMessages: []execution.LLMMessage{
				{Role: execution.RoleUser, Content: "Hello"},
				{Role: execution.RoleAssistant, Content: "Hi"},
			},
		}

		saved := executor.GetMessages()
		assert.Len(t, saved, 2)
		assert.Equal(t, execution.RoleUser, saved[0].Role)
		assert.Equal(t, execution.RoleAssistant, saved[1].Role)
	})
}

func TestNormalizeEnvVarExpr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: "",
		},
		{
			name:     "PlainVariableName",
			input:    "OPENAI_API_KEY",
			expected: "${OPENAI_API_KEY}",
		},
		{
			name:     "DollarPrefix",
			input:    "$ANTHROPIC_KEY",
			expected: "${ANTHROPIC_KEY}",
		},
		{
			name:     "BracedFormat",
			input:    "${MY_API_KEY}",
			expected: "${MY_API_KEY}",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := normalizeEnvVarExpr(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestNewChatExecutor(t *testing.T) {
	t.Parallel()

	t.Run("NilLLMConfig", func(t *testing.T) {
		t.Parallel()

		step := core.Step{Name: "test"}
		_, err := newChatExecutor(context.Background(), step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "llm configuration is required")
	})

	t.Run("InvalidProvider", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM:  &core.LLMConfig{Provider: "invalid-provider"},
		}
		_, err := newChatExecutor(context.Background(), step)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid provider")
	})

	t.Run("ValidConfigWithOpenAI", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM: &core.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4o",
			},
		}
		exec, err := newChatExecutor(context.Background(), step)
		require.NoError(t, err)
		assert.NotNil(t, exec)
	})

	t.Run("WithSystemMessage", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM: &core.LLMConfig{
				Provider: "anthropic",
				Model:    "claude-sonnet-4-20250514",
				System:   "You are a helpful assistant",
			},
		}
		exec, err := newChatExecutor(context.Background(), step)
		require.NoError(t, err)

		e := exec.(*Executor)
		assert.Len(t, e.messages, 1)
		assert.Equal(t, core.LLMRoleSystem, e.messages[0].Role)
		assert.Equal(t, "You are a helpful assistant", e.messages[0].Content)
	})

	t.Run("WithStepMessages", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM: &core.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4o",
			},
			Messages: []core.LLMMessage{
				{Role: core.LLMRoleUser, Content: "Hello"},
				{Role: core.LLMRoleAssistant, Content: "Hi there"},
			},
		}
		exec, err := newChatExecutor(context.Background(), step)
		require.NoError(t, err)

		e := exec.(*Executor)
		assert.Len(t, e.messages, 2)
		assert.Equal(t, core.LLMRoleUser, e.messages[0].Role)
		assert.Equal(t, core.LLMRoleAssistant, e.messages[1].Role)
	})

	t.Run("WithSystemAndStepMessages", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM: &core.LLMConfig{
				Provider: "gemini",
				Model:    "gemini-pro",
				System:   "Be concise",
			},
			Messages: []core.LLMMessage{
				{Role: core.LLMRoleUser, Content: "What is 2+2?"},
			},
		}
		exec, err := newChatExecutor(context.Background(), step)
		require.NoError(t, err)

		e := exec.(*Executor)
		assert.Len(t, e.messages, 2)
		assert.Equal(t, core.LLMRoleSystem, e.messages[0].Role)
		assert.Equal(t, core.LLMRoleUser, e.messages[1].Role)
	})

	t.Run("CustomAPIKeyName", func(t *testing.T) {
		t.Parallel()

		step := core.Step{
			Name: "test",
			LLM: &core.LLMConfig{
				Provider:   "openai",
				Model:      "gpt-4o",
				APIKeyName: "MY_CUSTOM_KEY",
			},
		}
		exec, err := newChatExecutor(context.Background(), step)
		require.NoError(t, err)

		e := exec.(*Executor)
		assert.Equal(t, "MY_CUSTOM_KEY", e.apiKeyEnvVar)
	})
}

func TestExecutor_SetStdout(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	e := &Executor{}
	e.SetStdout(&buf)
	assert.Equal(t, &buf, e.stdout)
}

func TestExecutor_SetStderr(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	e := &Executor{}
	e.SetStderr(&buf)
	assert.Equal(t, &buf, e.stderr)
}

func TestExecutor_Kill(t *testing.T) {
	t.Parallel()

	e := &Executor{}
	err := e.Kill(os.Interrupt)
	assert.NoError(t, err)
}

func TestToLLMMessages(t *testing.T) {
	t.Parallel()

	t.Run("EmptySlice", func(t *testing.T) {
		t.Parallel()

		result := toLLMMessages(nil)
		assert.Empty(t, result)
	})

	t.Run("ConvertMessages", func(t *testing.T) {
		t.Parallel()

		msgs := []execution.LLMMessage{
			{Role: execution.RoleSystem, Content: "System prompt"},
			{Role: execution.RoleUser, Content: "User message"},
			{Role: execution.RoleAssistant, Content: "Assistant response"},
		}
		result := toLLMMessages(msgs)

		assert.Len(t, result, 3)
		assert.Equal(t, llmpkg.RoleSystem, result[0].Role)
		assert.Equal(t, "System prompt", result[0].Content)
		assert.Equal(t, llmpkg.RoleUser, result[1].Role)
		assert.Equal(t, "User message", result[1].Content)
		assert.Equal(t, llmpkg.RoleAssistant, result[2].Role)
		assert.Equal(t, "Assistant response", result[2].Content)
	})
}

func TestBuildMessageList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		stepMsgs        []execution.LLMMessage
		contextMsgs     []execution.LLMMessage
		wantFirstSystem string
		wantLen         int
	}{
		{
			name: "step system takes precedence",
			stepMsgs: []execution.LLMMessage{
				{Role: execution.RoleSystem, Content: "step system"},
				{Role: execution.RoleUser, Content: "user"},
			},
			contextMsgs: []execution.LLMMessage{
				{Role: execution.RoleSystem, Content: "context system"},
			},
			wantFirstSystem: "step system",
			wantLen:         2,
		},
		{
			name:     "context system used when step has none",
			stepMsgs: []execution.LLMMessage{{Role: execution.RoleUser, Content: "user"}},
			contextMsgs: []execution.LLMMessage{
				{Role: execution.RoleSystem, Content: "context system"},
			},
			wantFirstSystem: "context system",
			wantLen:         2,
		},
		{
			name: "no context",
			stepMsgs: []execution.LLMMessage{
				{Role: execution.RoleSystem, Content: "step system"},
			},
			contextMsgs:     nil,
			wantFirstSystem: "step system",
			wantLen:         1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result := buildMessageList(tt.stepMsgs, tt.contextMsgs)

			require.Len(t, result, tt.wantLen)
			assert.Equal(t, execution.RoleSystem, result[0].Role)
			assert.Equal(t, tt.wantFirstSystem, result[0].Content)
		})
	}
}

func TestToThinkingRequest(t *testing.T) {
	t.Parallel()

	t.Run("NilConfig", func(t *testing.T) {
		t.Parallel()

		result := toThinkingRequest(nil)
		assert.Nil(t, result)
	})

	t.Run("WithConfig", func(t *testing.T) {
		t.Parallel()

		budget := 1024
		cfg := &core.ThinkingConfig{
			Enabled:         true,
			Effort:          core.ThinkingEffortHigh,
			BudgetTokens:    &budget,
			IncludeInOutput: true,
		}
		result := toThinkingRequest(cfg)

		require.NotNil(t, result)
		assert.True(t, result.Enabled)
		assert.Equal(t, llmpkg.ThinkingEffort("high"), result.Effort)
		assert.Equal(t, &budget, result.BudgetTokens)
		assert.True(t, result.IncludeInOutput)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		t.Parallel()

		cfg := &core.ThinkingConfig{}
		result := toThinkingRequest(cfg)

		require.NotNil(t, result)
		assert.False(t, result.Enabled)
		assert.Empty(t, result.Effort)
		assert.Nil(t, result.BudgetTokens)
		assert.False(t, result.IncludeInOutput)
	})
}
