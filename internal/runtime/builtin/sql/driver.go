package sql

import (
	"context"
	"database/sql"
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
}

// DriverRegistry holds registered database drivers.
type DriverRegistry struct {
	drivers map[string]Driver
}

// NewDriverRegistry creates a new driver registry.
func NewDriverRegistry() *DriverRegistry {
	return &DriverRegistry{
		drivers: make(map[string]Driver),
	}
}

// Register adds a driver to the registry.
func (r *DriverRegistry) Register(driver Driver) {
	r.drivers[driver.Name()] = driver
}

// Get retrieves a driver by name.
func (r *DriverRegistry) Get(name string) (Driver, bool) {
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
