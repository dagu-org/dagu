package chat

import (
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
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
