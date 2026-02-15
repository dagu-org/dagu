package agent

import (
	"context"
	"errors"
)

// Sentinel errors for session store operations.
var (
	ErrSessionNotFound  = errors.New("session not found")
	ErrInvalidSessionID = errors.New("invalid session ID")
	ErrInvalidUserID    = errors.New("invalid user ID")
)

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
