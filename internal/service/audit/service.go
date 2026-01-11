package audit

import (
	"context"
	"encoding/json"
)

// Store defines the interface for persisting audit entries.
type Store interface {
	// Append adds a new audit entry to the store.
	Append(ctx context.Context, entry *Entry) error
	// Query retrieves audit entries matching the filter.
	Query(ctx context.Context, filter QueryFilter) (*QueryResult, error)
}

// Service provides audit logging functionality.
type Service struct {
	store Store
}

// New creates a new audit service.
func New(store Store) *Service {
	return &Service{store: store}
}

// Log records an audit entry.
func (s *Service) Log(ctx context.Context, entry *Entry) error {
	return s.store.Append(ctx, entry)
}

// Query retrieves audit entries matching the filter.
func (s *Service) Query(ctx context.Context, filter QueryFilter) (*QueryResult, error) {
	return s.store.Query(ctx, filter)
}

// LogTerminalSessionStart logs the start of a terminal session.
func (s *Service) LogTerminalSessionStart(ctx context.Context, userID, username, sessionID, ipAddress string) error {
	details, _ := json.Marshal(map[string]string{"session_id": sessionID})
	entry := NewEntry(CategoryTerminal, "session_start", userID, username).
		WithDetails(string(details)).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}

// LogTerminalCommand logs a command executed in a terminal session.
func (s *Service) LogTerminalCommand(ctx context.Context, userID, username, sessionID, command, ipAddress string) error {
	details, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"command":    command,
	})
	entry := NewEntry(CategoryTerminal, "command", userID, username).
		WithDetails(string(details)).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}

// LogTerminalSessionEnd logs the end of a terminal session.
func (s *Service) LogTerminalSessionEnd(ctx context.Context, userID, username, sessionID, reason, ipAddress string) error {
	details, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"reason":     reason,
	})
	entry := NewEntry(CategoryTerminal, "session_end", userID, username).
		WithDetails(string(details)).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}
