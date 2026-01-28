// Package fileagentconfig provides a file-based storage for agent configuration.
package fileagentconfig

// Default values for agent configuration.
const (
	DefaultProvider = "anthropic"
	DefaultModel    = "claude-sonnet-4-5"
)

// AgentConfig holds the configuration for the AI agent feature.
type AgentConfig struct {
	Enabled bool           `json:"enabled"`
	LLM     AgentLLMConfig `json:"llm"`
}

// AgentLLMConfig holds LLM provider configuration for the agent.
// Supported providers: anthropic, openai, gemini, openrouter, or local.
type AgentLLMConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

// DefaultConfig returns the default agent configuration.
func DefaultConfig() *AgentConfig {
	return &AgentConfig{
		Enabled: true,
		LLM: AgentLLMConfig{
			Provider: DefaultProvider,
			Model:    DefaultModel,
		},
	}
}
