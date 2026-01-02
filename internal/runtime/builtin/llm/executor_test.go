package llm

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
)

func TestExecutor_MessageSaving(t *testing.T) {
	t.Parallel()

	t.Run("HistoryEnabled_SavesAllMessages", func(t *testing.T) {
		t.Parallel()

		historyEnabled := true
		executor := &Executor{
			step: core.Step{
				LLM: &core.LLMConfig{
					Provider: "openai",
					Model:    "gpt-4o",
					History:  &historyEnabled,
				},
			},
			messages: []execution.LLMMessage{
				{Role: "user", Content: "Hello"},
			},
			inheritedMessages: []execution.LLMMessage{
				{Role: "system", Content: "You are helpful"},
				{Role: "user", Content: "Previous question"},
				{Role: "assistant", Content: "Previous answer"},
			},
		}

		// Simulate what happens after Run() completes
		allMessages := append(executor.inheritedMessages, executor.messages...)
		metadata := &execution.LLMMessageMetadata{
			Provider:         "openai",
			Model:            "gpt-4o",
			PromptTokens:     10,
			CompletionTokens: 5,
			TotalTokens:      15,
		}
		executor.savedMessages = append(allMessages, execution.LLMMessage{
			Role:     "assistant",
			Content:  "Hello there!",
			Metadata: metadata,
		})

		saved := executor.GetMessages()
		assert.Len(t, saved, 5) // 3 inherited + 1 user + 1 assistant
		assert.Equal(t, "system", saved[0].Role)
		assert.Equal(t, "assistant", saved[4].Role)
		assert.NotNil(t, saved[4].Metadata)
		assert.Equal(t, 15, saved[4].Metadata.TotalTokens)
	})

	t.Run("HistoryDisabled_SavesOnlyStepMessages", func(t *testing.T) {
		t.Parallel()

		historyDisabled := false
		executor := &Executor{
			step: core.Step{
				LLM: &core.LLMConfig{
					Provider: "anthropic",
					Model:    "claude-sonnet-4-20250514",
					History:  &historyDisabled,
				},
			},
			messages: []execution.LLMMessage{
				{Role: "system", Content: "New system prompt"},
				{Role: "user", Content: "New question"},
			},
			inheritedMessages: []execution.LLMMessage{
				{Role: "system", Content: "Old system prompt"},
				{Role: "user", Content: "Old question"},
				{Role: "assistant", Content: "Old answer"},
			},
		}

		// Simulate what happens after Run() with history disabled
		// Should NOT include inherited messages
		metadata := &execution.LLMMessageMetadata{
			Provider:         "anthropic",
			Model:            "claude-sonnet-4-20250514",
			PromptTokens:     20,
			CompletionTokens: 10,
			TotalTokens:      30,
		}
		executor.savedMessages = append(executor.messages, execution.LLMMessage{
			Role:     "assistant",
			Content:  "New answer",
			Metadata: metadata,
		})

		saved := executor.GetMessages()
		assert.Len(t, saved, 3) // 2 step messages + 1 assistant (NO inherited)
		assert.Equal(t, "system", saved[0].Role)
		assert.Equal(t, "New system prompt", saved[0].Content)
		assert.Equal(t, "user", saved[1].Role)
		assert.Equal(t, "New question", saved[1].Content)
		assert.Equal(t, "assistant", saved[2].Role)
		assert.NotNil(t, saved[2].Metadata)
		assert.Equal(t, "anthropic", saved[2].Metadata.Provider)
		assert.Equal(t, 30, saved[2].Metadata.TotalTokens)
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
				{Role: "user", Content: "Test"},
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
			Role:     "assistant",
			Content:  "Response",
			Metadata: metadata,
		})

		saved := executor.GetMessages()
		assert.Len(t, saved, 2)

		assistantMsg := saved[1]
		assert.Equal(t, "assistant", assistantMsg.Role)
		assert.NotNil(t, assistantMsg.Metadata)
		assert.Equal(t, "gemini", assistantMsg.Metadata.Provider)
		assert.Equal(t, "gemini-pro", assistantMsg.Metadata.Model)
		assert.Equal(t, 5, assistantMsg.Metadata.PromptTokens)
		assert.Equal(t, 3, assistantMsg.Metadata.CompletionTokens)
		assert.Equal(t, 8, assistantMsg.Metadata.TotalTokens)
	})
}

func TestExecutor_SetInheritedMessages(t *testing.T) {
	t.Parallel()

	executor := &Executor{}

	messages := []execution.LLMMessage{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "User message"},
		{Role: "assistant", Content: "Assistant response"},
	}

	executor.SetInheritedMessages(messages)

	assert.Equal(t, messages, executor.inheritedMessages)
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
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi"},
			},
		}

		saved := executor.GetMessages()
		assert.Len(t, saved, 2)
		assert.Equal(t, "user", saved[0].Role)
		assert.Equal(t, "assistant", saved[1].Role)
	})
}
