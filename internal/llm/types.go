// Package llm provides a generic abstraction layer for interacting with
// Large Language Model providers.
package llm

import "time"

// Role represents the role of a message sender in a conversation.
type Role string

const (
	// RoleSystem represents a system message that sets behavior/context.
	RoleSystem Role = "system"
	// RoleUser represents a message from the user.
	RoleUser Role = "user"
	// RoleAssistant represents a message from the AI assistant.
	RoleAssistant Role = "assistant"
	// RoleTool represents a tool/function call result.
	RoleTool Role = "tool"
)

// ParseRole converts a string to a Role, with support for common aliases.
func ParseRole(s string) Role {
	switch s {
	case "system", "sys":
		return RoleSystem
	case "user", "human":
		return RoleUser
	case "assistant", "ai", "bot":
		return RoleAssistant
	case "tool", "function":
		return RoleTool
	default:
		return Role(s)
	}
}

// Message represents a single message in a conversation.
type Message struct {
	// Role identifies who sent the message (system, user, assistant, or tool).
	Role Role `json:"role"`
	// Content is the text content of the message.
	Content string `json:"content"`
	// Name is an optional identifier for the message sender.
	// Useful in multi-agent scenarios or for tool messages.
	Name string `json:"name,omitempty"`
	// ToolCallID is the ID of the tool call this message is responding to.
	// Required when Role is "tool".
	ToolCallID string `json:"tool_call_id,omitempty"`
}

// Usage contains token usage information from an LLM API call.
type Usage struct {
	// PromptTokens is the number of tokens in the prompt.
	PromptTokens int `json:"prompt_tokens"`
	// CompletionTokens is the number of tokens in the completion.
	CompletionTokens int `json:"completion_tokens"`
	// TotalTokens is the sum of prompt and completion tokens.
	TotalTokens int `json:"total_tokens"`
}

// Config contains configuration for an LLM provider.
type Config struct {
	// APIKey is the authentication key for the provider.
	APIKey string
	// BaseURL is the API endpoint URL. If empty, the provider's default is used.
	BaseURL string
	// Timeout is the maximum time to wait for a response.
	// Default is 60 seconds if not specified.
	Timeout time.Duration

	// Retry configuration
	// MaxRetries is the maximum number of retry attempts for transient errors.
	// Default is 3 if not specified.
	MaxRetries int
	// InitialInterval is the initial backoff interval before the first retry.
	// Default is 1 second if not specified.
	InitialInterval time.Duration
	// MaxInterval is the maximum backoff interval between retries.
	// Default is 30 seconds if not specified.
	MaxInterval time.Duration
	// Multiplier is the factor by which the interval increases after each retry.
	// Default is 2.0 if not specified.
	Multiplier float64
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Timeout:         60 * time.Second,
		MaxRetries:      3,
		InitialInterval: 1 * time.Second,
		MaxInterval:     30 * time.Second,
		Multiplier:      2.0,
	}
}

// ChatRequest contains the input for a chat completion request.
type ChatRequest struct {
	// Model is the identifier of the model to use.
	Model string
	// Messages is the conversation history to send to the model.
	Messages []Message
	// Temperature controls randomness in the response (0.0 to 2.0).
	// Lower values make output more deterministic.
	Temperature *float64
	// MaxTokens is the maximum number of tokens to generate.
	MaxTokens *int
	// TopP is the nucleus sampling parameter (0.0 to 1.0).
	TopP *float64
	// Stop is a list of sequences where the model will stop generating.
	Stop []string
}

// ChatResponse contains the output from a chat completion request.
type ChatResponse struct {
	// Content is the generated text content.
	Content string
	// FinishReason indicates why the model stopped generating.
	// Common values: "stop", "length", "content_filter".
	FinishReason string
	// Usage contains token usage statistics.
	Usage Usage
}

// StreamEvent represents a single event in a streaming response.
type StreamEvent struct {
	// Delta is the incremental content received in this event.
	Delta string
	// Done indicates whether the stream has completed.
	Done bool
	// Error contains any error that occurred during streaming.
	Error error
	// Usage contains token usage statistics (only set when Done is true).
	Usage *Usage
}
