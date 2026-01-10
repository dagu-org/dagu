// Package postgres provides the PostgreSQL driver for the SQL executor.
package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"hash/fnv"
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

// BuildInsertQuery generates a multi-row INSERT statement for PostgreSQL.
func (d *PostgresDriver) BuildInsertQuery(table string, columns []string, rowCount int, onConflict string) string {
	var sb strings.Builder

	sb.WriteString("INSERT INTO ")
	sb.WriteString(table)
	sb.WriteString(" (")
	sb.WriteString(strings.Join(columns, ", "))
	sb.WriteString(") VALUES ")

	paramIdx := 1
	for i := 0; i < rowCount; i++ {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(")
		for j := 0; j < len(columns); j++ {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("$%d", paramIdx))
			paramIdx++
		}
		sb.WriteString(")")
	}

	// ON CONFLICT handling
	// Note: "replace" requires knowing the conflict target (primary key or unique constraint),
	// which is not available at this level. Both "ignore" and "replace" use DO NOTHING.
	switch onConflict {
	case "ignore", "replace":
		sb.WriteString(" ON CONFLICT DO NOTHING")
	}

	return sb.String()
}

func init() {
	sqlexec.RegisterDriver(&PostgresDriver{})
}
