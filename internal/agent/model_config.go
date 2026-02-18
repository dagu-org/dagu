package agent

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Sentinel errors for model store operations.
var (
	ErrModelNotFound          = errors.New("model not found")
	ErrModelAlreadyExists     = errors.New("model already exists")
	ErrModelNameAlreadyExists = errors.New("model name already exists")
	ErrInvalidModelID         = errors.New("invalid model ID")
)

// Config holds the configuration for the AI agent feature.
type Config struct {
	Enabled        bool             `json:"enabled"`
	DefaultModelID string           `json:"defaultModelId,omitempty"`
	ToolPolicy     ToolPolicyConfig `json:"toolPolicy"`
	EnabledSkills  []string         `json:"enabledSkills,omitempty"`
}

// BashRuleAction is the decision a bash rule applies when matched.
type BashRuleAction string

const (
	BashRuleActionAllow BashRuleAction = "allow"
	BashRuleActionDeny  BashRuleAction = "deny"
)

// BashDefaultBehavior defines behavior when no rule matches.
type BashDefaultBehavior string

const (
	BashDefaultBehaviorAllow BashDefaultBehavior = "allow"
	BashDefaultBehaviorDeny  BashDefaultBehavior = "deny"
)

// BashDenyBehavior defines behavior when a command is denied by policy.
type BashDenyBehavior string

const (
	BashDenyBehaviorAskUser BashDenyBehavior = "ask_user"
	BashDenyBehaviorBlock   BashDenyBehavior = "block"
)

// BashRule defines an ordered regex rule used for bash command policy checks.
type BashRule struct {
	Name    string         `json:"name,omitempty"`
	Pattern string         `json:"pattern"`
	Action  BashRuleAction `json:"action"`
	Enabled *bool          `json:"enabled,omitempty"`
}

// BashPolicyConfig configures granular bash command policy behavior.
type BashPolicyConfig struct {
	Rules           []BashRule          `json:"rules,omitempty"`
	DefaultBehavior BashDefaultBehavior `json:"defaultBehavior,omitempty"`
	DenyBehavior    BashDenyBehavior    `json:"denyBehavior,omitempty"`
}

// ToolPolicyConfig configures agent tool permissions.
// Tools is a map of tool name to enabled/disabled state.
type ToolPolicyConfig struct {
	Tools map[string]bool  `json:"tools,omitempty"`
	Bash  BashPolicyConfig `json:"bash"`
}

// LLMConfig holds LLM provider configuration for the agent.
// Supported providers: anthropic, openai, gemini, openrouter, or local.
type LLMConfig struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"apiKey"`
	BaseURL  string `json:"baseUrl,omitempty"`
}

// DefaultConfig returns the default agent configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:    true,
		ToolPolicy: DefaultToolPolicy(),
	}
}

// DefaultToolPolicy returns the default agent tool policy.
// The default is permissive: all tools enabled and bash allowed unless rules deny.
// Tool names and default-enabled states are derived from the tool registry.
func DefaultToolPolicy() ToolPolicyConfig {
	regs := RegisteredTools()
	tools := make(map[string]bool, len(regs))
	for _, reg := range regs {
		tools[reg.Name] = reg.DefaultEnabled
	}
	return ToolPolicyConfig{
		Tools: tools,
		Bash: BashPolicyConfig{
			Rules:           []BashRule{},
			DefaultBehavior: BashDefaultBehaviorAllow,
			DenyBehavior:    BashDenyBehaviorAskUser,
		},
	}
}

// ModelConfig holds the configuration for a single LLM model.
type ModelConfig struct {
	ID               string  `json:"id"`
	Name             string  `json:"name"`
	Provider         string  `json:"provider"`
	Model            string  `json:"model"`
	APIKey           string  `json:"apiKey,omitempty"`
	BaseURL          string  `json:"baseUrl,omitempty"`
	ContextWindow    int     `json:"contextWindow,omitempty"`
	MaxOutputTokens  int     `json:"maxOutputTokens,omitempty"`
	InputCostPer1M   float64 `json:"inputCostPer1M,omitempty"`
	OutputCostPer1M  float64 `json:"outputCostPer1M,omitempty"`
	SupportsThinking bool    `json:"supportsThinking,omitempty"`
	Description      string  `json:"description,omitempty"`
}

// ToLLMConfig converts a ModelConfig to an LLMConfig for provider creation.
func (m *ModelConfig) ToLLMConfig() LLMConfig {
	return LLMConfig{
		Provider: m.Provider,
		Model:    m.Model,
		APIKey:   m.APIKey,
		BaseURL:  m.BaseURL,
	}
}

// validModelIDRegexp matches a valid model ID slug: lowercase alphanumeric segments separated by hyphens.
var validModelIDRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxModelIDLength = 128

// ValidateModelID validates that id is a safe, well-formed model identifier.
// It must be a non-empty slug (lowercase alphanumeric segments separated by hyphens)
// and at most 128 characters. This prevents path traversal and other injection attacks.
func ValidateModelID(id string) error {
	if id == "" {
		return ErrInvalidModelID
	}
	if len(id) > maxModelIDLength {
		return fmt.Errorf("%w: exceeds maximum length of %d", ErrInvalidModelID, maxModelIDLength)
	}
	if !validModelIDRegexp.MatchString(id) {
		return fmt.Errorf("%w: must match pattern [a-z0-9]+(-[a-z0-9]+)*", ErrInvalidModelID)
	}
	return nil
}

var slugRegexp = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlugID creates a URL-friendly slug from a name.
// E.g., "Claude Opus 4.6" -> "claude-opus-4-6"
func GenerateSlugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// maxSuffixLen reserves room for collision suffixes like "-999999999".
const maxSuffixLen = 10

// UniqueID generates a unique slug ID, appending "-2", "-3" etc. on collision.
// The fallback is used when the name produces an empty slug (e.g. only special characters).
// The result is guaranteed to not exceed maxModelIDLength.
func UniqueID(name string, existingIDs map[string]struct{}, fallback string) string {
	base := GenerateSlugID(name)
	if base == "" {
		base = fallback
	}
	if len(base) > maxModelIDLength-maxSuffixLen {
		base = base[:maxModelIDLength-maxSuffixLen]
	}
	id := base
	if _, exists := existingIDs[id]; !exists {
		return id
	}
	for i := 2; ; i++ {
		id = fmt.Sprintf("%s-%d", base, i)
		if _, exists := existingIDs[id]; !exists {
			return id
		}
	}
}

// MemoryContent holds loaded memory for system prompt injection.
type MemoryContent struct {
	GlobalMemory string // Contents of global MEMORY.md (truncated)
	DAGMemory    string // Contents of per-DAG MEMORY.md (truncated)
	DAGName      string // Name of the DAG (empty if no DAG context)
	MemoryDir    string // Root memory directory path
}
