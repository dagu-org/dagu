package agent

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/llm"
)

// Sentinel errors for store operations.
var (
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrInvalidConversationID = errors.New("invalid conversation ID")
	ErrInvalidUserID         = errors.New("invalid user ID")
)

// Default configuration values.
const (
	DefaultProvider = "anthropic"
	DefaultModel    = "claude-sonnet-4-5"
)

// Config holds the configuration for the AI agent feature.
type Config struct {
	Enabled bool      `json:"enabled"`
	LLM     LLMConfig `json:"llm"`
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
		LLM: LLMConfig{
			Provider: DefaultProvider,
			Model:    DefaultModel,
		},
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
	// GetProvider returns the LLM provider and model name.
	GetProvider(ctx context.Context) (llm.Provider, string, error)
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
