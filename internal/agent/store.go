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
	// Conversation errors.
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrInvalidConversationID = errors.New("invalid conversation ID")
	ErrInvalidUserID         = errors.New("invalid user ID")

	// Model errors.
	ErrModelNotFound      = errors.New("model not found")
	ErrModelAlreadyExists = errors.New("model already exists")
	ErrInvalidModelID     = errors.New("invalid model ID")
)

// Config holds the configuration for the AI agent feature.
type Config struct {
	Enabled        bool   `json:"enabled"`
	DefaultModelID string `json:"defaultModelId,omitempty"`
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
		Enabled: true,
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

// UniqueID generates a unique slug ID, appending "-2", "-3" etc. on collision.
func UniqueID(name string, existingIDs map[string]struct{}) string {
	base := GenerateSlugID(name)
	if base == "" {
		base = "model"
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

// ConversationStore defines the interface for conversation persistence.
// All implementations must be safe for concurrent use.
type ConversationStore interface {
	// CreateConversation creates a new conversation.
	// Returns ErrInvalidConversationID if conv.ID is empty.
	// Returns ErrInvalidUserID if conv.UserID is empty.
	CreateConversation(ctx context.Context, conv *Conversation) error

	// GetConversation retrieves a conversation by ID.
	// Returns ErrInvalidConversationID if id is empty.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetConversation(ctx context.Context, id string) (*Conversation, error)

	// ListConversations returns all conversations for a user, sorted by UpdatedAt descending.
	// Returns ErrInvalidUserID if userID is empty.
	// Returns an empty slice if the user has no conversations.
	ListConversations(ctx context.Context, userID string) ([]*Conversation, error)

	// UpdateConversation updates conversation metadata such as Title and UpdatedAt.
	// Returns ErrConversationNotFound if the conversation does not exist.
	UpdateConversation(ctx context.Context, conv *Conversation) error

	// DeleteConversation removes a conversation and all its messages.
	// Returns ErrInvalidConversationID if id is empty.
	// Returns ErrConversationNotFound if the conversation does not exist.
	DeleteConversation(ctx context.Context, id string) error

	// AddMessage appends a message to a conversation and updates the conversation's UpdatedAt.
	// Returns ErrInvalidConversationID if conversationID is empty.
	// Returns ErrConversationNotFound if the conversation does not exist.
	AddMessage(ctx context.Context, conversationID string, msg *Message) error

	// GetMessages retrieves all messages for a conversation, ordered by SequenceID ascending.
	// Returns ErrInvalidConversationID if conversationID is empty.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetMessages(ctx context.Context, conversationID string) ([]Message, error)

	// GetLatestSequenceID returns the highest sequence ID for a conversation.
	// Returns 0 if the conversation has no messages.
	// Returns ErrInvalidConversationID if conversationID is empty.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetLatestSequenceID(ctx context.Context, conversationID string) (int64, error)
}
