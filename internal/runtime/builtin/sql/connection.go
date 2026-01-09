package sql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

// ConnectionManager manages database connections with pooling.
type ConnectionManager struct {
	mu       sync.Mutex
	db       *sql.DB
	driver   Driver
	cfg      *Config
	cleanup  func() error
	refCount int
}

// NewConnectionManager creates a new connection manager.
func NewConnectionManager(ctx context.Context, driver Driver, cfg *Config) (*ConnectionManager, error) {
	db, cleanup, err := driver.Connect(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)

	// Verify connection
	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		if cleanup != nil {
			_ = cleanup()
		}
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &ConnectionManager{
		db:       db,
		driver:   driver,
		cfg:      cfg,
		cleanup:  cleanup,
		refCount: 1,
	}, nil
}

// DB returns the underlying database connection.
func (m *ConnectionManager) DB() *sql.DB {
	return m.db
}

// Driver returns the database driver.
func (m *ConnectionManager) Driver() Driver {
	return m.driver
}

// Config returns the configuration.
func (m *ConnectionManager) Config() *Config {
	return m.cfg
}

// Acquire increments the reference count.
func (m *ConnectionManager) Acquire() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.refCount++
}

// Release decrements the reference count and closes if zero.
func (m *ConnectionManager) Release() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.refCount--
	if m.refCount <= 0 {
		return m.closeInternal()
	}
	return nil
}

// Close closes the connection manager.
func (m *ConnectionManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeInternal()
}

func (m *ConnectionManager) closeInternal() error {
	var errs []error

	if m.cleanup != nil {
		if err := m.cleanup(); err != nil {
			errs = append(errs, fmt.Errorf("cleanup error: %w", err))
		}
	}

	if m.db != nil {
		if err := m.db.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close error: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	return nil
}

// Transaction represents a database transaction with isolation level support.
type Transaction struct {
	tx             *sql.Tx
	isolationLevel sql.IsolationLevel
}

// BeginTransaction starts a new transaction with the specified isolation level.
func BeginTransaction(ctx context.Context, db *sql.DB, isolationLevel string) (*Transaction, error) {
	level, err := parseIsolationLevel(isolationLevel)
	if err != nil {
		return nil, err
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: level,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	return &Transaction{
		tx:             tx,
		isolationLevel: level,
	}, nil
}

// Tx returns the underlying transaction.
func (t *Transaction) Tx() *sql.Tx {
	return t.tx
}

// Commit commits the transaction.
func (t *Transaction) Commit() error {
	return t.tx.Commit()
}

// Rollback rolls back the transaction.
func (t *Transaction) Rollback() error {
	return t.tx.Rollback()
}

// parseIsolationLevel converts a string isolation level to sql.IsolationLevel.
func parseIsolationLevel(level string) (sql.IsolationLevel, error) {
	switch level {
	case "", "default":
		return sql.LevelDefault, nil
	case "read_committed":
		return sql.LevelReadCommitted, nil
	case "repeatable_read":
		return sql.LevelRepeatableRead, nil
	case "serializable":
		return sql.LevelSerializable, nil
	default:
		return sql.LevelDefault, fmt.Errorf("unknown isolation level: %s", level)
	}
}

// QueryExecutor provides methods to execute SQL queries.
type QueryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

// GetQueryExecutor returns the appropriate query executor (transaction or db).
func GetQueryExecutor(db *sql.DB, tx *Transaction) QueryExecutor {
	if tx != nil {
		return tx.Tx()
	}
	return db
}
