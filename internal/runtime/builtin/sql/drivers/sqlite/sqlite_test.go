package sqlite

import (
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

func TestSQLiteDriver_BuildInsertQuery(t *testing.T) {
	driver := &SQLiteDriver{}

	tests := []struct {
		name       string
		table      string
		columns    []string
		rowCount   int
		onConflict string
		want       string
	}{
		{
			name:       "single row",
			table:      "users",
			columns:    []string{"name", "age"},
			rowCount:   1,
			onConflict: "error",
			want:       "INSERT INTO users (name, age) VALUES (?, ?)",
		},
		{
			name:       "multiple rows",
			table:      "users",
			columns:    []string{"name", "age"},
			rowCount:   3,
			onConflict: "error",
			want:       "INSERT INTO users (name, age) VALUES (?, ?), (?, ?), (?, ?)",
		},
		{
			name:       "with ignore",
			table:      "users",
			columns:    []string{"id", "name"},
			rowCount:   2,
			onConflict: "ignore",
			want:       "INSERT OR IGNORE INTO users (id, name) VALUES (?, ?), (?, ?)",
		},
		{
			name:       "with replace",
			table:      "users",
			columns:    []string{"id", "name"},
			rowCount:   1,
			onConflict: "replace",
			want:       "INSERT OR REPLACE INTO users (id, name) VALUES (?, ?)",
		},
		{
			name:       "single column",
			table:      "items",
			columns:    []string{"value"},
			rowCount:   2,
			onConflict: "error",
			want:       "INSERT INTO items (value) VALUES (?), (?)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driver.BuildInsertQuery(tt.table, tt.columns, tt.rowCount, tt.onConflict)
			assert.Equal(t, tt.want, got)
		})
	}
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

