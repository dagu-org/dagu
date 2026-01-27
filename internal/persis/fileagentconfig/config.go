// Package fileagentconfig provides a file-based storage for agent configuration.
package fileagentconfig

// AgentConfig holds the configuration for the AI agent feature.
type AgentConfig struct {
	// Enabled indicates whether the agent feature is enabled.
	Enabled bool `json:"enabled"`

	// LLM contains the LLM provider configuration.
	LLM AgentLLMConfig `json:"llm"`
}

// AgentLLMConfig holds LLM provider configuration for the agent.
type AgentLLMConfig struct {
	// Provider is the LLM provider type (anthropic, openai, gemini, openrouter, local).
	Provider string `json:"provider"`

	// Model is the model ID to use.
	Model string `json:"model"`

	// APIKey is the API key for the LLM provider.
	APIKey string `json:"apiKey"`

	// BaseURL is the optional custom API endpoint URL.
	BaseURL string `json:"baseUrl,omitempty"`
}

// DefaultConfig returns the default agent configuration.
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		Enabled: false,
		LLM: AgentLLMConfig{
			Provider: "anthropic",
			Model:    "claude-sonnet-4-20250514",
		},
	}
}
