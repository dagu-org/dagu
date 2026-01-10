package sqlite

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSQLiteDriver_Name(t *testing.T) {
	driver := &SQLiteDriver{}
	assert.Equal(t, "sqlite", driver.Name())
}

func TestSQLiteDriver_PlaceholderFormat(t *testing.T) {
	driver := &SQLiteDriver{}
	assert.Equal(t, "?", driver.PlaceholderFormat())
}

func TestSQLiteDriver_SupportsAdvisoryLock(t *testing.T) {
	driver := &SQLiteDriver{}
	assert.False(t, driver.SupportsAdvisoryLock())
}

func TestSQLiteDriver_AcquireAdvisoryLock(t *testing.T) {
	driver := &SQLiteDriver{}
	_, err := driver.AcquireAdvisoryLock(nil, nil, "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestSQLiteDriver_ConvertNamedParams(t *testing.T) {
	driver := &SQLiteDriver{}

	tests := []struct {
		name        string
		query       string
		params      map[string]any
		wantQuery   string
		wantParams  []any
		wantErr     bool
	}{
		{
			name:        "single parameter",
			query:       "SELECT * FROM users WHERE id = :id",
			params:      map[string]any{"id": 123},
			wantQuery:   "SELECT * FROM users WHERE id = ?",
			wantParams:  []any{123},
			wantErr:     false,
		},
		{
			name:        "multiple parameters",
			query:       "SELECT * FROM users WHERE name = :name AND status = :status",
			params:      map[string]any{"name": "Alice", "status": "active"},
			wantQuery:   "SELECT * FROM users WHERE name = ? AND status = ?",
			wantParams:  []any{"Alice", "active"},
			wantErr:     false,
		},
		{
			name:        "repeated parameter",
			query:       "SELECT * FROM users WHERE id = :id OR parent_id = :id",
			params:      map[string]any{"id": 123},
			wantQuery:   "SELECT * FROM users WHERE id = ? OR parent_id = ?",
			wantParams:  []any{123, 123}, // Each ? needs its own parameter
			wantErr:     false,
		},
		{
			name:        "no parameters in query",
			query:       "SELECT * FROM users",
			params:      map[string]any{"id": 123},
			wantQuery:   "SELECT * FROM users",
			wantParams:  nil,
			wantErr:     false,
		},
		{
			name:        "missing parameter",
			query:       "SELECT * FROM users WHERE id = :id",
			params:      map[string]any{"other": 123},
			wantQuery:   "",
			wantParams:  nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotParams, err := driver.ConvertNamedParams(tt.query, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantQuery, gotQuery)
			assert.Equal(t, tt.wantParams, gotParams)
		})
	}
}

func TestIsMemoryDB(t *testing.T) {
	tests := []struct {
		dsn  string
		want bool
	}{
		{":memory:", true},
		{"mode=memory", true},
		{"file:./test.db?mode=memory", true},
		{"./test.db", false},
		{"file:./test.db", false},
		{"file:./test.db?mode=rwc", false},
	}

	for _, tt := range tests {
		t.Run(tt.dsn, func(t *testing.T) {
			got := isMemoryDB(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractDBPath(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want string
	}{
		{
			name: "plain file path",
			dsn:  "./test.db",
			want: "./test.db",
		},
		{
			name: "file prefix",
			dsn:  "file:./test.db",
			want: "./test.db",
		},
		{
			name: "file prefix with params",
			dsn:  "file:./test.db?mode=rwc",
			want: "./test.db",
		},
		{
			name: "plain path with params",
			dsn:  "./test.db?_busy_timeout=5000",
			want: "./test.db",
		},
		{
			name: "absolute path",
			dsn:  "/var/data/test.db",
			want: "/var/data/test.db",
		},
		{
			name: "absolute path with file prefix",
			dsn:  "file:/var/data/test.db",
			want: "/var/data/test.db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDBPath(tt.dsn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEnsureDBDir(t *testing.T) {
	t.Run("memory database", func(t *testing.T) {
		err := EnsureDBDir(":memory:")
		assert.NoError(t, err)
	})

	t.Run("create directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		dbPath := filepath.Join(tmpDir, "subdir", "test.db")

		err := EnsureDBDir(dbPath)
		assert.NoError(t, err)

		// Verify directory was created
		dir := filepath.Dir(dbPath)
		info, err := os.Stat(dir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})
}

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "single statement",
			script: "SELECT 1",
			want:   []string{"SELECT 1"},
		},
		{
			name:   "multiple statements",
			script: "SELECT 1; SELECT 2; SELECT 3",
			want:   []string{"SELECT 1", "SELECT 2", "SELECT 3"},
		},
		{
			name:   "statement with semicolon in string",
			script: "SELECT 'hello; world'",
			want:   []string{"SELECT 'hello; world'"},
		},
		{
			name:   "empty statements",
			script: "SELECT 1;; SELECT 2",
			want:   []string{"SELECT 1", "SELECT 2"},
		},
		{
			name:   "trailing semicolon",
			script: "SELECT 1;",
			want:   []string{"SELECT 1"},
		},
		{
			name:   "whitespace only",
			script: "   ;   ",
			want:   nil,
		},
		{
			name:   "escaped single quote",
			script: "SELECT 'it''s okay'",
			want:   []string{"SELECT 'it''s okay'"},
		},
		{
			name:   "double quotes",
			script: `SELECT "column; name"`,
			want:   []string{`SELECT "column; name"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitStatements(tt.script)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestIsSelectQuery(t *testing.T) {
	tests := []struct {
		query string
		want  bool
	}{
		{"SELECT * FROM users", true},
		{"  SELECT * FROM users", true},
		{"select * from users", true},
		{"WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"VALUES (1, 2), (3, 4)", true},
		{"PRAGMA table_info(users)", true},
		{"INSERT INTO users VALUES (1)", false},
		{"UPDATE users SET name = 'foo'", false},
		{"DELETE FROM users", false},
		{"CREATE TABLE users (id INT)", false},
		{"DROP TABLE users", false},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := IsSelectQuery(tt.query)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestFormatError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name:     "database is locked",
			err:      errWithMessage("database is locked"),
			contains: "sqlite database is locked",
		},
		{
			name:     "no such table",
			err:      errWithMessage("no such table: foo"),
			contains: "sqlite table not found",
		},
		{
			name:     "no such column",
			err:      errWithMessage("no such column: bar"),
			contains: "sqlite column not found",
		},
		{
			name:     "syntax error",
			err:      errWithMessage("syntax error near"),
			contains: "sqlite syntax error",
		},
		{
			name:     "UNIQUE constraint failed",
			err:      errWithMessage("UNIQUE constraint failed: users.id"),
			contains: "sqlite unique constraint violation",
		},
		{
			name:     "FOREIGN KEY constraint failed",
			err:      errWithMessage("FOREIGN KEY constraint failed"),
			contains: "sqlite foreign key violation",
		},
		{
			name:     "NOT NULL constraint failed",
			err:      errWithMessage("NOT NULL constraint failed: users.name"),
			contains: "sqlite not null constraint violation",
		},
		{
			name:     "unable to open database",
			err:      errWithMessage("unable to open database file"),
			contains: "sqlite unable to open database",
		},
		{
			name:     "unknown error",
			err:      errWithMessage("some unknown error"),
			contains: "some unknown error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatError(tt.err)
			if tt.err == nil {
				assert.Nil(t, result)
				return
			}
			assert.Contains(t, result.Error(), tt.contains)
		})
	}
}

func TestMapExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "database is locked",
			err:  errWithMessage("database is locked"),
			want: 5,
		},
		{
			name: "unable to open",
			err:  errWithMessage("unable to open database"),
			want: 2,
		},
		{
			name: "syntax error",
			err:  errWithMessage("syntax error near"),
			want: 4,
		},
		{
			name: "constraint violation",
			err:  errWithMessage("UNIQUE constraint failed"),
			want: 6,
		},
		{
			name: "no such table",
			err:  errWithMessage("no such table: foo"),
			want: 4,
		},
		{
			name: "no such column",
			err:  errWithMessage("no such column: bar"),
			want: 4,
		},
		{
			name: "unknown error",
			err:  errWithMessage("some unknown error"),
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapExitCode(tt.err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildDSN(t *testing.T) {
	tests := []struct {
		name        string
		filePath    string
		mode        string
		journal     string
		busyTimeout int
		want        string
	}{
		{
			name:        "memory database",
			filePath:    ":memory:",
			mode:        "",
			journal:     "",
			busyTimeout: 0,
			want:        ":memory:",
		},
		{
			name:        "simple file path",
			filePath:    "./test.db",
			mode:        "",
			journal:     "",
			busyTimeout: 0,
			want:        "file:./test.db",
		},
		{
			name:        "with mode",
			filePath:    "./test.db",
			mode:        "rwc",
			journal:     "",
			busyTimeout: 0,
			want:        "file:./test.db?mode=rwc",
		},
		{
			name:        "with journal mode",
			filePath:    "./test.db",
			mode:        "",
			journal:     "WAL",
			busyTimeout: 0,
			want:        "file:./test.db?_journal_mode=WAL",
		},
		{
			name:        "with busy timeout",
			filePath:    "./test.db",
			mode:        "",
			journal:     "",
			busyTimeout: 5000,
			want:        "file:./test.db?_busy_timeout=5000",
		},
		{
			name:        "full configuration",
			filePath:    "./test.db",
			mode:        "rwc",
			journal:     "WAL",
			busyTimeout: 5000,
			want:        "file:./test.db?mode=rwc&_journal_mode=WAL&_busy_timeout=5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDSN(tt.filePath, tt.mode, tt.journal, tt.busyTimeout)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create errors with specific messages
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func errWithMessage(msg string) error {
	return &testError{msg: msg}
}
