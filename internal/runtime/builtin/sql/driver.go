package sql

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"sync"
)

// Driver defines the interface for SQL database drivers.
// Each database (PostgreSQL, SQLite) implements this interface.
type Driver interface {
	// Name returns the driver identifier (e.g., "postgres", "sqlite")
	Name() string

	// Connect establishes a connection to the database using the provided configuration.
	// Returns a *sql.DB instance and any error encountered.
	Connect(ctx context.Context, cfg *Config) (*sql.DB, func() error, error)

	// SupportsAdvisoryLock indicates if the driver supports advisory locking.
	SupportsAdvisoryLock() bool

	// AcquireAdvisoryLock acquires a named advisory lock (PostgreSQL-specific).
	// Returns a release function and any error.
	AcquireAdvisoryLock(ctx context.Context, db *sql.DB, lockName string) (func() error, error)

	// ConvertNamedParams converts named parameters (:param) to driver-specific format ($1, ?).
	// Returns the converted query and ordered parameter values.
	ConvertNamedParams(query string, params map[string]any) (string, []any, error)

	// PlaceholderFormat returns the placeholder format for the driver ("$" for PostgreSQL, "?" for SQLite).
	PlaceholderFormat() string

	// BuildInsertQuery generates a multi-row INSERT statement for batch imports.
	// table: target table name
	// columns: column names to insert
	// rowCount: number of rows in this batch
	// onConflict: conflict handling strategy ("error", "ignore", "replace")
	// conflictTarget: column(s) for conflict detection (required for PostgreSQL UPSERT with "replace")
	// updateColumns: columns to update on conflict (if empty, updates all non-key columns)
	// Returns the SQL query string with placeholders.
	BuildInsertQuery(table string, columns []string, rowCount int, onConflict, conflictTarget string, updateColumns []string) string

	// QuoteIdentifier quotes a table or column name to handle reserved words and special characters.
	// For PostgreSQL and SQLite, this wraps the identifier in double quotes.
	QuoteIdentifier(name string) string
}

// DriverRegistry holds registered database drivers with thread-safe access.
type DriverRegistry struct {
	mu      sync.RWMutex
	drivers map[string]Driver
}

// NewDriverRegistry creates a new driver registry.
func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{
		drivers: make(map[string]Driver),
	}
}

// Register adds a driver to the registry (thread-safe).
func (r *DriverRegistry) Register(driver Driver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drivers[driver.Name()] = driver
}

// Get retrieves a driver by name (thread-safe).
func (r *DriverRegistry) Get(name string) (Driver, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	driver, ok := r.drivers[name]
	return driver, ok
}

// globalRegistry is the default driver registry.
var globalRegistry = NewDriverRegistry()

// RegisterDriver registers a driver in the global registry.
func RegisterDriver(driver Driver) {
	globalRegistry.Register(driver)
}

// GetDriver retrieves a driver from the global registry.
func GetDriver(name string) (Driver, bool) {
	return globalRegistry.Get(name)
}

// QuoteIdentifier quotes a table or column name to handle reserved words and special characters.
// This is a shared implementation for SQL databases that use double quotes (PostgreSQL, SQLite).
// Any existing double quotes in the identifier are escaped by doubling them.
func QuoteIdentifier(name string) string {
	escaped := strings.ReplaceAll(name, `"`, `""`)
	return `"` + escaped + `"`
}

// ParseConflictTarget extracts column names from a conflict target string.
// Handles both single column "id" and composite "(user_id, org_id)" formats.
func ParseConflictTarget(target string) []string {
	target = strings.TrimSpace(target)
	// Remove surrounding parentheses if present
	target = strings.TrimPrefix(target, "(")
	target = strings.TrimSuffix(target, ")")

	parts := strings.Split(target, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Remove quotes if present
		p = strings.Trim(p, `"`)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// Contains checks if a string slice contains a specific string.
func Contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
