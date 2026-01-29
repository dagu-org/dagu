package agent

import (
	"context"
	"errors"
)

// Sentinel errors for conversation store operations.
var (
	ErrConversationNotFound  = errors.New("conversation not found")
	ErrInvalidConversationID = errors.New("conversation ID is invalid")
	ErrInvalidUserID         = errors.New("user ID is invalid")
)

// ConversationStore defines the interface for conversation persistence.
// All implementations must be safe for concurrent use.
type ConversationStore interface {
	// CreateConversation creates a new conversation.
	// The conversation must have a valid ID and UserID.
	CreateConversation(ctx context.Context, conv *Conversation) error

	// GetConversation retrieves a conversation by ID.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetConversation(ctx context.Context, id string) (*Conversation, error)

	// ListConversations returns all conversations for a user, sorted by UpdatedAt descending.
	// Returns an empty slice if no conversations exist.
	ListConversations(ctx context.Context, userID string) ([]*Conversation, error)

	// UpdateConversation updates conversation metadata (e.g., UpdatedAt, Title).
	// Returns ErrConversationNotFound if the conversation does not exist.
	UpdateConversation(ctx context.Context, conv *Conversation) error

	// DeleteConversation removes a conversation and all its messages.
	// Returns ErrConversationNotFound if the conversation does not exist.
	DeleteConversation(ctx context.Context, id string) error

	// AddMessage appends a message to a conversation.
	// Also updates the conversation's UpdatedAt timestamp.
	// Returns ErrConversationNotFound if the conversation does not exist.
	AddMessage(ctx context.Context, conversationID string, msg *Message) error

	// GetMessages retrieves all messages for a conversation, ordered by SequenceID.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetMessages(ctx context.Context, conversationID string) ([]Message, error)

	// GetLatestSequenceID returns the highest sequence ID for a conversation.
	// Returns 0 if the conversation has no messages.
	// Returns ErrConversationNotFound if the conversation does not exist.
	GetLatestSequenceID(ctx context.Context, conversationID string) (int64, error)
}
