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
	CategoryAgent    Category = "agent"
)

// Entry represents a single audit log entry.
type Entry struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Category  Category  `json:"category"`
	Action    string    `json:"action"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Details   string    `json:"details,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
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
	Category  Category
	UserID    string
	StartTime time.Time // inclusive
	EndTime   time.Time // exclusive
	Limit     int
	Offset    int
}

// QueryResult contains the result of a query.
type QueryResult struct {
	Entries []*Entry `json:"entries"`
	Total   int      `json:"total"` // total before pagination
}
