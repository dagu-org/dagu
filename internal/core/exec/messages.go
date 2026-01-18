package exec

import "github.com/dagu-org/dagu/internal/core"

// LLM message role constants - aliases for core package constants.
const (
	RoleSystem    = core.LLMRoleSystem
	RoleUser      = core.LLMRoleUser
	RoleAssistant = core.LLMRoleAssistant
	RoleTool      = core.LLMRoleTool
)

// LLMMessage represents a single message in the conversation.
type LLMMessage struct {
	// Role is the message role (system, user, assistant, tool).
	Role core.LLMRole `json:"role"`
	// Content is the message content.
	Content string `json:"content"`
	// ToolCallID is the ID of the tool call this message is responding to.
	// Only set when Role is "tool".
	ToolCallID string `json:"tool_call_id,omitempty"`
	// Metadata contains API call metadata (only set for assistant responses).
	Metadata *LLMMessageMetadata `json:"metadata,omitempty"`
}

// LLMMessageMetadata contains metadata about an LLM API call.
type LLMMessageMetadata struct {
	// Provider is the LLM provider used (openai, anthropic, etc.).
	Provider string `json:"provider,omitempty"`
	// Model is the model identifier used.
	Model string `json:"model,omitempty"`
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"promptTokens,omitempty"`
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completionTokens,omitempty"`
	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int `json:"totalTokens,omitempty"`
}

// DeduplicateSystemMessages keeps only the first system message.
func DeduplicateSystemMessages(messages []LLMMessage) []LLMMessage {
	if len(messages) == 0 {
		return nil
	}

	result := make([]LLMMessage, 0, len(messages))
	seenSystem := false

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			if seenSystem {
				continue
			}
			seenSystem = true
		}
		result = append(result, msg)
	}

	return result
}
