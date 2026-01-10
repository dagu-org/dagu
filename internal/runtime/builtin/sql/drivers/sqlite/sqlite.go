// Package sqlite provides the SQLite driver for the SQL executor.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"

	sqlexec "github.com/dagu-org/dagu/internal/runtime/builtin/sql"
	"github.com/gofrs/flock"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// SQLiteDriver implements the Driver interface for SQLite.
type SQLiteDriver struct {
	// Track file locks to ensure proper cleanup
	locks   map[string]*flock.Flock
	locksMu sync.Mutex
}

var sqliteDriverInstance = &SQLiteDriver{
	locks: make(map[string]*flock.Flock),
}

// Name returns the driver name.
func (d *SQLiteDriver) Name() string {
	return "sqlite"
}

// Connect establishes a connection to SQLite.
func (d *SQLiteDriver) Connect(ctx context.Context, cfg *sqlexec.Config) (*sql.DB, func() error, error) {
	dsn := cfg.DSN
	var cleanup func() error

	// Handle file locking if requested
	if cfg.FileLock && !isMemoryDB(dsn) {
		dbPath := extractDBPath(dsn)
		lockPath := dbPath + ".lock"

		d.locksMu.Lock()
		fl := flock.New(lockPath)
		d.locksMu.Unlock()

		locked, err := fl.TryLock()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to acquire file lock: %w", err)
		}
		if !locked {
			return nil, nil, fmt.Errorf("database is locked by another process")
		}

		d.locksMu.Lock()
		d.locks[lockPath] = fl
		d.locksMu.Unlock()

		cleanup = func() error {
			d.locksMu.Lock()
			defer d.locksMu.Unlock()
			if fl, ok := d.locks[lockPath]; ok {
				delete(d.locks, lockPath)
				return fl.Unlock()
			}
			return nil
		}
	}

	// Open SQLite connection
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		return nil, nil, fmt.Errorf("failed to open sqlite connection: %w", err)
	}

	// Set pragmas for robustness
	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA busy_timeout = 5000",
	}

	for _, pragma := range pragmas {
		if _, err := db.ExecContext(ctx, pragma); err != nil {
			_ = db.Close()
			if cleanup != nil {
				_ = cleanup()
			}
			return nil, nil, fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	// Wrap cleanup to also close the database
	finalCleanup := func() error {
		if cleanup != nil {
			return cleanup()
		}
		return nil
	}

	return db, finalCleanup, nil
}

// SupportsAdvisoryLock returns false as SQLite doesn't support advisory locks.
func (d *SQLiteDriver) SupportsAdvisoryLock() bool {
	return false
}

// AcquireAdvisoryLock is not supported for SQLite.
func (d *SQLiteDriver) AcquireAdvisoryLock(_ context.Context, _ *sql.DB, _ string) (func() error, error) {
	return nil, fmt.Errorf("advisory locks are not supported for SQLite")
}

// ConvertNamedParams converts named parameters to SQLite positional format.
func (d *SQLiteDriver) ConvertNamedParams(query string, params map[string]any) (string, []any, error) {
	return sqlexec.ConvertNamedToPositional(query, params, "?")
}

// PlaceholderFormat returns the SQLite placeholder format.
func (d *SQLiteDriver) PlaceholderFormat() string {
	return "?"
}

// BuildInsertQuery generates a multi-row INSERT statement for SQLite.
func (d *SQLiteDriver) BuildInsertQuery(table string, columns []string, rowCount int, onConflict string) string {
	var sb strings.Builder

	// SQLite uses INSERT OR IGNORE / INSERT OR REPLACE
	switch onConflict {
	case "ignore":
		sb.WriteString("INSERT OR IGNORE INTO ")
	case "replace":
		sb.WriteString("INSERT OR REPLACE INTO ")
	default:
		sb.WriteString("INSERT INTO ")
	}

	sb.WriteString(table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(columns, ", "))
	sb.WriteString(") VALUES ")

	for i := 0; i < rowCount; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for j := 0; j < len(columns); j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString("?")
		}
		sb.WriteString(")")
	}

	return sb.String()
}

// isMemoryDB checks if the DSN is for an in-memory database.
func isMemoryDB(dsn string) bool {
	return dsn == ":memory:" || strings.Contains(dsn, "mode=memory")
}

// extractDBPath extracts the file path from a SQLite DSN.
func extractDBPath(dsn string) string {
	// Handle "file:" prefix
	if strings.HasPrefix(dsn, "file:") {
		path := strings.TrimPrefix(dsn, "file:")
		// Remove query parameters
		if idx := strings.Index(path, "?"); idx >= 0 {
			path = path[:idx]
		}
		return path
	}
	// Plain file path
	if idx := strings.Index(dsn, "?"); idx >= 0 {
		return dsn[:idx]
	}
	return dsn
}

func init() {
	sqlexec.RegisterDriver(sqliteDriverInstance)
}
