package core

import "fmt"

// LLMRole represents the role of a message sender in a conversation.
type LLMRole string

// LLM message role constants.
const (
	LLMRoleSystem    LLMRole = "system"
	LLMRoleUser      LLMRole = "user"
	LLMRoleAssistant LLMRole = "assistant"
	LLMRoleTool      LLMRole = "tool"
)

// ParseLLMRole validates and returns an LLMRole from a string.
// Returns error for invalid or empty role values.
func ParseLLMRole(s string) (LLMRole, error) {
	switch LLMRole(s) {
	case LLMRoleSystem, LLMRoleUser, LLMRoleAssistant, LLMRoleTool:
		return LLMRole(s), nil
	default:
		return "", fmt.Errorf("invalid role %q: must be one of: system, user, assistant, tool", s)
	}
}

// ThinkingEffort represents the reasoning depth level for thinking mode.
type ThinkingEffort string

// ThinkingEffort constants for reasoning/thinking depth.
const (
	ThinkingEffortLow    ThinkingEffort = "low"
	ThinkingEffortMedium ThinkingEffort = "medium"
	ThinkingEffortHigh   ThinkingEffort = "high"
	ThinkingEffortXHigh  ThinkingEffort = "xhigh"
)

// ParseThinkingEffort validates and returns a ThinkingEffort from a string.
// Returns empty string for empty input (no effort specified).
// Returns error for invalid effort values.
func ParseThinkingEffort(s string) (ThinkingEffort, error) {
	if s == "" {
		return "", nil
	}
	switch ThinkingEffort(s) {
	case ThinkingEffortLow, ThinkingEffortMedium, ThinkingEffortHigh, ThinkingEffortXHigh:
		return ThinkingEffort(s), nil
	default:
		return "", fmt.Errorf("invalid thinking effort %q: must be one of: low, medium, high, xhigh", s)
	}
}

// ThinkingConfig contains configuration for extended thinking/reasoning.
type ThinkingConfig struct {
	// Enabled activates thinking mode for supported models.
	Enabled bool `json:"enabled,omitempty"`
	// Effort controls reasoning depth: low, medium, high, xhigh.
	// Maps to provider-specific parameters.
	Effort ThinkingEffort `json:"effort,omitempty"`
	// BudgetTokens sets explicit token budget (provider-specific).
	// For Anthropic: minimum 1024, max 128K.
	// For Gemini 2.5: range 128-32768.
	BudgetTokens *int `json:"budgetTokens,omitempty"`
	// IncludeInOutput includes thinking blocks in stdout.
	// Default is false for consistency across providers.
	IncludeInOutput bool `json:"includeInOutput,omitempty"`
}

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
	// Thinking enables extended thinking/reasoning mode.
	// Provider-specific: Anthropic uses budget_tokens, OpenAI uses reasoning.effort,
	// Gemini uses thinkingLevel/thinkingBudget, OpenRouter normalizes across providers.
	Thinking *ThinkingConfig `json:"thinking,omitempty"`
}

// LLMMessage represents a message in the LLM conversation.
type LLMMessage struct {
	// Role is the message role (system, user, assistant, tool).
	Role LLMRole `json:"role,omitempty"`
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
