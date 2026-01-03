package core

// LLM message role constants.
const (
	LLMRoleSystem    = "system"
	LLMRoleUser      = "user"
	LLMRoleAssistant = "assistant"
	LLMRoleTool      = "tool"
)

// LLMConfig contains the configuration for LLM-based executors (chat, agent, etc.).
type LLMConfig struct {
	// Provider is the LLM provider (openai, anthropic, gemini, openrouter, local).
	Provider string `json:"provider,omitempty"`
	// Model is the model to use (e.g., gpt-4o, claude-sonnet-4-20250514).
	Model string `json:"model,omitempty"`
	// System is the default system prompt for conversations.
	System string `json:"system,omitempty"`
	// Temperature controls randomness (0.0-2.0).
	Temperature *float64 `json:"temperature,omitempty"`
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens *int `json:"maxTokens,omitempty"`
	// TopP is the nucleus sampling parameter.
	TopP *float64 `json:"topP,omitempty"`
	// BaseURL is a custom API endpoint.
	BaseURL string `json:"baseURL,omitempty"`
	// APIKeyName is the name of the environment variable containing the API key.
	// If not specified, the default environment variable for the provider is used.
	APIKeyName string `json:"apiKeyName,omitempty"`
	// Stream enables or disables streaming output.
	// Default is true.
	Stream *bool `json:"stream,omitempty"`
}

// LLMMessage represents a message in the LLM conversation.
type LLMMessage struct {
	// Role is the message role (system, user, assistant, tool).
	Role string `json:"role,omitempty"`
	// Content is the message content. Supports variable substitution with ${VAR}.
	Content string `json:"content,omitempty"`
}

// StreamEnabled returns true if streaming is enabled.
// Default is true if Stream is nil.
func (c *LLMConfig) StreamEnabled() bool {
	if c.Stream == nil {
		return true
	}
	return *c.Stream
}

// ExecutorTypeChat is the executor type for chat steps.
const ExecutorTypeChat = "chat"
