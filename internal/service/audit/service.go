package audit

import (
	"context"
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
	entry := NewEntry(CategoryTerminal, "session_start", userID, username).
		WithDetails(`{"session_id":"` + sessionID + `"}`).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}

// LogTerminalCommand logs a command executed in a terminal session.
func (s *Service) LogTerminalCommand(ctx context.Context, userID, username, sessionID, command, ipAddress string) error {
	entry := NewEntry(CategoryTerminal, "command", userID, username).
		WithDetails(`{"session_id":"` + sessionID + `","command":"` + escapeJSON(command) + `"}`).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}

// LogTerminalSessionEnd logs the end of a terminal session.
func (s *Service) LogTerminalSessionEnd(ctx context.Context, userID, username, sessionID, reason, ipAddress string) error {
	entry := NewEntry(CategoryTerminal, "session_end", userID, username).
		WithDetails(`{"session_id":"` + sessionID + `","reason":"` + reason + `"}`).
		WithIPAddress(ipAddress)
	return s.Log(ctx, entry)
}

// escapeJSON escapes special characters for JSON strings.
func escapeJSON(s string) string {
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			result = append(result, '\\', '"')
		case '\\':
			result = append(result, '\\', '\\')
		case '\n':
			result = append(result, '\\', 'n')
		case '\r':
			result = append(result, '\\', 'r')
		case '\t':
			result = append(result, '\\', 't')
		default:
			if c < 32 {
				// Skip other control characters
				continue
			}
			result = append(result, c)
		}
	}
	return string(result)
}
