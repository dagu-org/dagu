package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Sentinel errors for store operations.
var (
	// Session errors.
	ErrSessionNotFound  = errors.New("session not found")
	ErrInvalidSessionID = errors.New("invalid session ID")
	ErrInvalidUserID    = errors.New("invalid user ID")

	// Model errors.
	ErrModelNotFound          = errors.New("model not found")
	ErrModelAlreadyExists     = errors.New("model already exists")
	ErrModelNameAlreadyExists = errors.New("model name already exists")
	ErrInvalidModelID         = errors.New("invalid model ID")
)

// Config holds the configuration for the AI agent feature.
type Config struct {
	Enabled        bool             `json:"enabled"`
	DefaultModelID string           `json:"defaultModelId,omitempty"`
	ToolPolicy     ToolPolicyConfig `json:"toolPolicy,omitempty"`
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
	Bash  BashPolicyConfig `json:"bash,omitempty"`
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
// The default is strict for mutating/networking tools and deny-by-default for bash,
// with user approval fallback on denied bash commands.
func DefaultToolPolicy() ToolPolicyConfig {
	return ToolPolicyConfig{
		Tools: map[string]bool{
			"bash":        true,
			"read":        true,
			"patch":       false,
			"think":       true,
			"navigate":    true,
			"read_schema": true,
			"ask_user":    false,
			"web_search":  false,
		},
		Bash: BashPolicyConfig{
			Rules:           []BashRule{},
			DefaultBehavior: BashDefaultBehaviorDeny,
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
// E.g., "Claude Opus 4.6" → "claude-opus-4-6"
func GenerateSlugID(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRegexp.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

// maxSuffixLen reserves room for collision suffixes like "-999999999".
const maxSuffixLen = 10

// UniqueID generates a unique slug ID, appending "-2", "-3" etc. on collision.
// The result is guaranteed to not exceed maxModelIDLength.
func UniqueID(name string, existingIDs map[string]struct{}) string {
	base := GenerateSlugID(name)
	if base == "" {
		base = "model"
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

// ConfigStore provides access to agent configuration.
// All implementations must be safe for concurrent use.
type ConfigStore interface {
	// Load reads the agent configuration.
	Load(ctx context.Context) (*Config, error)
	// Save writes the agent configuration.
	Save(ctx context.Context, cfg *Config) error
	// IsEnabled returns whether the agent feature is enabled.
	IsEnabled(ctx context.Context) bool
}

// ModelStore defines the interface for model configuration persistence.
// All implementations must be safe for concurrent use.
type ModelStore interface {
	Create(ctx context.Context, model *ModelConfig) error
	GetByID(ctx context.Context, id string) (*ModelConfig, error)
	List(ctx context.Context) ([]*ModelConfig, error)
	Update(ctx context.Context, model *ModelConfig) error
	Delete(ctx context.Context, id string) error
}

// SessionStore defines the interface for session persistence.
// All implementations must be safe for concurrent use.
type SessionStore interface {
	// CreateSession creates a new session.
	// Returns ErrInvalidSessionID if sess.ID is empty.
	// Returns ErrInvalidUserID if sess.UserID is empty.
	CreateSession(ctx context.Context, sess *Session) error

	// GetSession retrieves a session by ID.
	// Returns ErrInvalidSessionID if id is empty.
	// Returns ErrSessionNotFound if the session does not exist.
	GetSession(ctx context.Context, id string) (*Session, error)

	// ListSessions returns all sessions for a user, sorted by UpdatedAt descending.
	// Returns ErrInvalidUserID if userID is empty.
	// Returns an empty slice if the user has no sessions.
	ListSessions(ctx context.Context, userID string) ([]*Session, error)

	// UpdateSession updates session metadata such as Title and UpdatedAt.
	// Returns ErrSessionNotFound if the session does not exist.
	UpdateSession(ctx context.Context, sess *Session) error

	// DeleteSession removes a session and all its messages.
	// Returns ErrInvalidSessionID if id is empty.
	// Returns ErrSessionNotFound if the session does not exist.
	DeleteSession(ctx context.Context, id string) error

	// AddMessage appends a message to a session and updates the session's UpdatedAt.
	// Returns ErrInvalidSessionID if sessionID is empty.
	// Returns ErrSessionNotFound if the session does not exist.
	AddMessage(ctx context.Context, sessionID string, msg *Message) error

	// GetMessages retrieves all messages for a session, ordered by SequenceID ascending.
	// Returns ErrInvalidSessionID if sessionID is empty.
	// Returns ErrSessionNotFound if the session does not exist.
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)

	// GetLatestSequenceID returns the highest sequence ID for a session.
	// Returns 0 if the session has no messages.
	// Returns ErrInvalidSessionID if sessionID is empty.
	// Returns ErrSessionNotFound if the session does not exist.
	GetLatestSequenceID(ctx context.Context, sessionID string) (int64, error)
}
