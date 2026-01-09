// Package sqlite provides the SQLite driver for the SQL executor.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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

// EnsureDBDir ensures the directory for the database file exists.
func EnsureDBDir(dsn string) error {
	if isMemoryDB(dsn) {
		return nil
	}
	dbPath := extractDBPath(dsn)
	dir := filepath.Dir(dbPath)
	return os.MkdirAll(dir, 0o755)
}

// SplitStatements splits SQL script into individual statements.
func SplitStatements(script string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)

	for i, r := range script {
		// Handle strings
		if (r == '\'' || r == '"') && !inString {
			inString = true
			stringChar = r
			current.WriteRune(r)
			continue
		}

		if inString {
			current.WriteRune(r)
			// Check for escaped quote
			if r == stringChar {
				if i+1 < len(script) && rune(script[i+1]) == stringChar {
					// Skip next character (escaped quote)
				} else {
					inString = false
				}
			}
			continue
		}

		// Handle statement terminator
		if r == ';' {
			stmt := strings.TrimSpace(current.String())
			if stmt != "" {
				statements = append(statements, stmt)
			}
			current.Reset()
			continue
		}

		current.WriteRune(r)
	}

	// Handle final statement without semicolon
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}

// IsSelectQuery checks if a query is a SELECT statement.
func IsSelectQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(trimmed, "SELECT") ||
		strings.HasPrefix(trimmed, "WITH") ||
		strings.HasPrefix(trimmed, "VALUES") ||
		strings.HasPrefix(trimmed, "PRAGMA")
}

// FormatError formats SQLite-specific errors.
func FormatError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "database is locked"):
		return fmt.Errorf("sqlite database is locked: %w", err)
	case strings.Contains(errStr, "no such table"):
		return fmt.Errorf("sqlite table not found: %w", err)
	case strings.Contains(errStr, "no such column"):
		return fmt.Errorf("sqlite column not found: %w", err)
	case strings.Contains(errStr, "syntax error"):
		return fmt.Errorf("sqlite syntax error: %w", err)
	case strings.Contains(errStr, "UNIQUE constraint failed"):
		return fmt.Errorf("sqlite unique constraint violation: %w", err)
	case strings.Contains(errStr, "FOREIGN KEY constraint failed"):
		return fmt.Errorf("sqlite foreign key violation: %w", err)
	case strings.Contains(errStr, "NOT NULL constraint failed"):
		return fmt.Errorf("sqlite not null constraint violation: %w", err)
	case strings.Contains(errStr, "unable to open database"):
		return fmt.Errorf("sqlite unable to open database: %w", err)
	default:
		return err
	}
}

// MapExitCode maps SQLite errors to exit codes.
func MapExitCode(err error) int {
	if err == nil {
		return 0
	}

	errStr := err.Error()

	switch {
	case strings.Contains(errStr, "database is locked"):
		return 5 // Database locked
	case strings.Contains(errStr, "unable to open"):
		return 2 // Connection error
	case strings.Contains(errStr, "syntax error"):
		return 4 // Syntax error
	case strings.Contains(errStr, "constraint"):
		return 6 // Constraint violation
	case strings.Contains(errStr, "no such"):
		return 4 // Object not found
	default:
		return 1 // Generic error
	}
}

// BuildDSN constructs a SQLite DSN from a file path and options.
func BuildDSN(filePath string, mode string, journal string, busyTimeout int) string {
	if filePath == ":memory:" {
		return filePath
	}

	var dsn strings.Builder
	dsn.WriteString("file:")
	dsn.WriteString(filePath)

	params := make([]string, 0)

	if mode != "" {
		params = append(params, "mode="+mode)
	}
	if journal != "" {
		params = append(params, "_journal_mode="+journal)
	}
	if busyTimeout > 0 {
		params = append(params, fmt.Sprintf("_busy_timeout=%d", busyTimeout))
	}

	if len(params) > 0 {
		dsn.WriteString("?")
		dsn.WriteString(strings.Join(params, "&"))
	}

	return dsn.String()
}

func init() {
	sqlexec.RegisterDriver(sqliteDriverInstance)
}
