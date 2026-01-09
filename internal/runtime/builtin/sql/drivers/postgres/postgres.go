// Package postgres provides the PostgreSQL driver for the SQL executor.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"

	sqlexec "github.com/dagu-org/dagu/internal/runtime/builtin/sql"

	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver
)

// PostgresDriver implements the Driver interface for PostgreSQL.
type PostgresDriver struct{}

// Name returns the driver name.
func (d *PostgresDriver) Name() string {
	return "postgres"
}

// Connect establishes a connection to PostgreSQL.
func (d *PostgresDriver) Connect(ctx context.Context, cfg *sqlexec.Config) (*sql.DB, func() error, error) {
	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	return db, nil, nil
}

// SupportsAdvisoryLock returns true as PostgreSQL supports advisory locks.
func (d *PostgresDriver) SupportsAdvisoryLock() bool {
	return true
}

// AcquireAdvisoryLock acquires a named advisory lock in PostgreSQL.
func (d *PostgresDriver) AcquireAdvisoryLock(ctx context.Context, db *sql.DB, lockName string) (func() error, error) {
	// Convert lock name to a 64-bit integer using FNV hash
	h := fnv.New64a()
	h.Write([]byte(lockName))
	lockID := int64(h.Sum64())

	// Acquire the advisory lock (blocks until acquired)
	_, err := db.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire advisory lock %q: %w", lockName, err)
	}

	// Return release function
	release := func() error {
		_, err := db.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", lockID)
		if err != nil {
			return fmt.Errorf("failed to release advisory lock %q: %w", lockName, err)
		}
		return nil
	}

	return release, nil
}

// ConvertNamedParams converts named parameters to PostgreSQL positional format.
func (d *PostgresDriver) ConvertNamedParams(query string, params map[string]any) (string, []any, error) {
	return sqlexec.ConvertNamedToPositional(query, params, "$")
}

// PlaceholderFormat returns the PostgreSQL placeholder format.
func (d *PostgresDriver) PlaceholderFormat() string {
	return "$"
}

// SplitStatements splits SQL script into individual statements.
// Handles semicolons inside strings and dollar-quoted strings.
func SplitStatements(script string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	stringChar := rune(0)
	inDollarQuote := false
	dollarTag := ""

	runes := []rune(script)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		// Handle dollar-quoted strings (PostgreSQL specific)
		if !inString && r == '$' {
			// Look for closing tag
			tagEnd := i + 1
			for tagEnd < len(runes) && (runes[tagEnd] == '_' || (runes[tagEnd] >= 'a' && runes[tagEnd] <= 'z') || (runes[tagEnd] >= 'A' && runes[tagEnd] <= 'Z') || (runes[tagEnd] >= '0' && runes[tagEnd] <= '9')) {
				tagEnd++
			}
			if tagEnd < len(runes) && runes[tagEnd] == '$' {
				tag := string(runes[i : tagEnd+1])
				if inDollarQuote && tag == dollarTag {
					inDollarQuote = false
					dollarTag = ""
				} else if !inDollarQuote {
					inDollarQuote = true
					dollarTag = tag
				}
				current.WriteString(tag)
				i = tagEnd
				continue
			}
		}

		// Skip if inside dollar-quoted string
		if inDollarQuote {
			current.WriteRune(r)
			continue
		}

		// Handle regular strings
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
				if i+1 < len(runes) && runes[i+1] == stringChar {
					current.WriteRune(runes[i+1])
					i++
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
		strings.HasPrefix(trimmed, "TABLE") ||
		strings.HasPrefix(trimmed, "VALUES")
}

// FormatError formats PostgreSQL-specific errors.
func FormatError(err error) error {
	if err == nil {
		return nil
	}

	errStr := err.Error()

	// Check for common PostgreSQL error patterns
	switch {
	case strings.Contains(errStr, "connection refused"):
		return fmt.Errorf("postgres connection refused: %w", err)
	case strings.Contains(errStr, "password authentication failed"):
		return fmt.Errorf("postgres authentication failed: %w", err)
	case strings.Contains(errStr, "does not exist"):
		return fmt.Errorf("postgres object not found: %w", err)
	case strings.Contains(errStr, "syntax error"):
		return fmt.Errorf("postgres syntax error: %w", err)
	case strings.Contains(errStr, "permission denied"):
		return fmt.Errorf("postgres permission denied: %w", err)
	case strings.Contains(errStr, "duplicate key"):
		return fmt.Errorf("postgres unique constraint violation: %w", err)
	case strings.Contains(errStr, "violates foreign key"):
		return fmt.Errorf("postgres foreign key violation: %w", err)
	default:
		return err
	}
}

// ExtractErrorCode extracts the SQLSTATE error code from a PostgreSQL error.
func ExtractErrorCode(err error) string {
	if err == nil {
		return ""
	}
	// pgx wraps errors - try to extract the code
	errStr := err.Error()
	// Look for SQLSTATE pattern
	if idx := strings.Index(errStr, "SQLSTATE"); idx >= 0 {
		end := idx + 13 // "SQLSTATE " + 5 char code
		if end <= len(errStr) {
			return errStr[idx+9 : end]
		}
	}
	return ""
}

// MapExitCode maps PostgreSQL errors to exit codes.
func MapExitCode(err error) int {
	if err == nil {
		return 0
	}

	code := ExtractErrorCode(err)
	errStr := err.Error()

	// Map SQLSTATE codes to exit codes
	switch {
	case code == "":
		// No SQLSTATE, check error message
		if strings.Contains(errStr, "connection") {
			return 2 // Connection error
		}
		return 1 // Generic error
	case strings.HasPrefix(code, "08"):
		return 2 // Connection exception
	case strings.HasPrefix(code, "28"):
		return 3 // Invalid authorization
	case strings.HasPrefix(code, "42"):
		return 4 // Syntax error or access violation
	case strings.HasPrefix(code, "23"):
		return 6 // Integrity constraint violation
	case strings.HasPrefix(code, "57"):
		return 7 // Operator intervention (timeout)
	default:
		return 1 // Generic SQL error
	}
}

// BuildDSN constructs a PostgreSQL DSN from components.
func BuildDSN(host string, port int, user, password, database string, sslmode string) string {
	var dsn strings.Builder
	dsn.WriteString("postgres://")

	if user != "" {
		dsn.WriteString(user)
		if password != "" {
			dsn.WriteString(":")
			dsn.WriteString(password)
		}
		dsn.WriteString("@")
	}

	dsn.WriteString(host)
	if port > 0 {
		dsn.WriteString(":")
		dsn.WriteString(strconv.Itoa(port))
	}

	dsn.WriteString("/")
	dsn.WriteString(database)

	if sslmode != "" {
		dsn.WriteString("?sslmode=")
		dsn.WriteString(sslmode)
	}

	return dsn.String()
}

func init() {
	sqlexec.RegisterDriver(&PostgresDriver{})
}
