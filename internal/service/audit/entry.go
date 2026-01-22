// Package audit provides a generic audit logging system for tracking user actions.
package audit

import (
	"time"

	"github.com/google/uuid"
)

// Category represents the type of feature being audited.
type Category string

// Predefined audit categories.
const (
	CategoryTerminal Category = "terminal"
	CategoryUser     Category = "user"
	CategoryDAG      Category = "dag"
	CategoryAPIKey   Category = "api_key"
	CategoryWebhook  Category = "webhook"
	CategoryGitSync  Category = "git_sync"
)

// Entry represents a single audit log entry.
type Entry struct {
	// ID is a unique identifier for this entry.
	ID string `json:"id"`
	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`
	// Category identifies the feature (e.g., "terminal", "user", "dag").
	Category Category `json:"category"`
	// Action describes what happened (e.g., "session_start", "command", "login").
	Action string `json:"action"`
	// UserID is the ID of the user who performed the action.
	UserID string `json:"user_id"`
	// Username is the username of the user who performed the action.
	Username string `json:"username"`
	// Details contains action-specific data as a JSON string.
	Details string `json:"details,omitempty"`
	// IPAddress is the client IP address if available.
	IPAddress string `json:"ip_address,omitempty"`
}

// NewEntry creates a new audit entry with a generated ID and current timestamp.
func NewEntry(category Category, action, userID, username string) *Entry {
	return &Entry{
		ID:        uuid.New().String(),
		Timestamp: time.Now().UTC(),
		Category:  category,
		Action:    action,
		UserID:    userID,
		Username:  username,
	}
}

// WithDetails adds details to the entry.
func (e *Entry) WithDetails(details string) *Entry {
	e.Details = details
	return e
}

// WithIPAddress adds an IP address to the entry.
func (e *Entry) WithIPAddress(ip string) *Entry {
	e.IPAddress = ip
	return e
}

// QueryFilter defines filters for querying audit entries.
type QueryFilter struct {
	// Category filters by audit category.
	Category Category
	// UserID filters by user ID.
	UserID string
	// StartTime filters entries after this time (inclusive).
	StartTime time.Time
	// EndTime filters entries before this time (exclusive).
	EndTime time.Time
	// Limit is the maximum number of entries to return.
	Limit int
	// Offset is the number of entries to skip.
	Offset int
}

// QueryResult contains the result of a query.
type QueryResult struct {
	// Entries is the list of audit entries matching the filter.
	Entries []*Entry `json:"entries"`
	// Total is the total number of entries matching the filter (before pagination).
	Total int `json:"total"`
}
