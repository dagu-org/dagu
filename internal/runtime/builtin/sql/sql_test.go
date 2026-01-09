package sql_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	sqlexec "github.com/dagu-org/dagu/internal/runtime/builtin/sql"
	// Import drivers for testing
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql/drivers/postgres"
	_ "github.com/dagu-org/dagu/internal/runtime/builtin/sql/drivers/sqlite"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		wantErr bool
		check   func(*testing.T, *sqlexec.Config)
	}{
		{
			name: "basic config",
			config: map[string]any{
				"dsn": "postgres://localhost/test",
			},
			wantErr: false,
			check: func(t *testing.T, cfg *sqlexec.Config) {
				assert.Equal(t, "postgres://localhost/test", cfg.DSN)
				assert.Equal(t, "jsonl", cfg.OutputFormat)
				assert.Equal(t, 60, cfg.Timeout)
			},
		},
		{
			name: "full config",
			config: map[string]any{
				"dsn":            "postgres://localhost/test",
				"timeout":        120,
				"transaction":    true,
				"isolationLevel": "serializable",
				"outputFormat":   "csv",
				"headers":        true,
				"maxRows":        1000,
			},
			wantErr: false,
			check: func(t *testing.T, cfg *sqlexec.Config) {
				assert.Equal(t, 120, cfg.Timeout)
				assert.True(t, cfg.Transaction)
				assert.Equal(t, "serializable", cfg.IsolationLevel)
				assert.Equal(t, "csv", cfg.OutputFormat)
				assert.True(t, cfg.Headers)
				assert.Equal(t, 1000, cfg.MaxRows)
			},
		},
		{
			name:    "missing dsn",
			config:  map[string]any{},
			wantErr: true,
		},
		{
			name: "invalid output format",
			config: map[string]any{
				"dsn":          "postgres://localhost/test",
				"outputFormat": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := sqlexec.ParseConfig(context.Background(), tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, cfg)
			}
		})
	}
}

func TestConvertNamedToPositional(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		params      map[string]any
		placeholder string
		wantQuery   string
		wantParams  []any
		wantErr     bool
	}{
		{
			name:        "single parameter",
			query:       "SELECT * FROM users WHERE id = :id",
			params:      map[string]any{"id": 123},
			placeholder: "$",
			wantQuery:   "SELECT * FROM users WHERE id = $1",
			wantParams:  []any{123},
			wantErr:     false,
		},
		{
			name:        "multiple parameters",
			query:       "SELECT * FROM users WHERE name = :name AND status = :status",
			params:      map[string]any{"name": "Alice", "status": "active"},
			placeholder: "$",
			wantQuery:   "SELECT * FROM users WHERE name = $1 AND status = $2",
			wantParams:  []any{"Alice", "active"},
			wantErr:     false,
		},
		{
			name:        "repeated parameter",
			query:       "SELECT * FROM users WHERE id = :id OR parent_id = :id",
			params:      map[string]any{"id": 123},
			placeholder: "$",
			wantQuery:   "SELECT * FROM users WHERE id = $1 OR parent_id = $1",
			wantParams:  []any{123},
			wantErr:     false,
		},
		{
			name:        "question mark placeholder",
			query:       "SELECT * FROM users WHERE id = :id",
			params:      map[string]any{"id": 123},
			placeholder: "?",
			wantQuery:   "SELECT * FROM users WHERE id = ?",
			wantParams:  []any{123},
			wantErr:     false,
		},
		{
			name:        "missing parameter",
			query:       "SELECT * FROM users WHERE id = :id",
			params:      map[string]any{"other": 123},
			placeholder: "$",
			wantErr:     true,
		},
		{
			name:        "no parameters in query",
			query:       "SELECT * FROM users",
			params:      map[string]any{"id": 123},
			placeholder: "$",
			wantQuery:   "SELECT * FROM users",
			wantParams:  nil,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQuery, gotParams, err := sqlexec.ConvertNamedToPositional(tt.query, tt.params, tt.placeholder)
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

func TestExtractParamNames(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  []string
	}{
		{
			name:  "single param",
			query: "SELECT * FROM users WHERE id = :id",
			want:  []string{"id"},
		},
		{
			name:  "multiple params",
			query: "SELECT * FROM users WHERE name = :name AND status = :status",
			want:  []string{"name", "status"},
		},
		{
			name:  "repeated params",
			query: "SELECT * FROM users WHERE id = :id OR parent_id = :id",
			want:  []string{"id"},
		},
		{
			name:  "no params",
			query: "SELECT * FROM users",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sqlexec.ExtractParamNames(tt.query)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestJSONLWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := sqlexec.NewJSONLWriter(&buf, "null")

	err := writer.WriteHeader([]string{"id", "name", "amount"})
	require.NoError(t, err)

	err = writer.WriteRow([]any{1, "Alice", 100.5})
	require.NoError(t, err)

	err = writer.WriteRow([]any{2, "Bob", nil})
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"id":1`)
	assert.Contains(t, output, `"name":"Alice"`)
	assert.Contains(t, output, `"amount":100.5`)
	assert.Contains(t, output, `"name":"Bob"`)
	assert.Contains(t, output, `"amount":null`)
}

func TestCSVWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := sqlexec.NewCSVWriter(&buf, "NULL", true)

	err := writer.WriteHeader([]string{"id", "name"})
	require.NoError(t, err)

	err = writer.WriteRow([]any{1, "Alice"})
	require.NoError(t, err)

	err = writer.WriteRow([]any{2, nil})
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "id,name")
	assert.Contains(t, output, "1,Alice")
	assert.Contains(t, output, "2,NULL")
}

func TestJSONWriter(t *testing.T) {
	var buf bytes.Buffer
	writer := sqlexec.NewJSONWriter(&buf, "null")

	err := writer.WriteHeader([]string{"id", "name"})
	require.NoError(t, err)

	err = writer.WriteRow([]any{1, "Alice"})
	require.NoError(t, err)

	err = writer.WriteRow([]any{2, "Bob"})
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"id": 1`)
	assert.Contains(t, output, `"name": "Alice"`)
	assert.Contains(t, output, `"name": "Bob"`)
	// Should be an array
	assert.True(t, output[0] == '[')
}

func TestDriverRegistry(t *testing.T) {
	// Test that drivers are registered
	postgres, ok := sqlexec.GetDriver("postgres")
	assert.True(t, ok)
	assert.NotNil(t, postgres)
	assert.Equal(t, "postgres", postgres.Name())

	sqlite, ok := sqlexec.GetDriver("sqlite")
	assert.True(t, ok)
	assert.NotNil(t, sqlite)
	assert.Equal(t, "sqlite", sqlite.Name())

	// Test unknown driver
	_, ok = sqlexec.GetDriver("unknown")
	assert.False(t, ok)
}

func TestSanitizeIdentifier(t *testing.T) {
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"users", false},
		{"user_table", false},
		{"schema.table", false},
		{"Table123", false},
		{"123table", true},
		{"user-table", true},
		{"user;table", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := sqlexec.SanitizeIdentifier(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateParams(t *testing.T) {
	tests := []struct {
		name    string
		query   string
		params  map[string]any
		wantErr bool
	}{
		{
			name:    "all params provided",
			query:   "SELECT * FROM users WHERE id = :id AND name = :name",
			params:  map[string]any{"id": 1, "name": "Alice"},
			wantErr: false,
		},
		{
			name:    "missing param",
			query:   "SELECT * FROM users WHERE id = :id AND name = :name",
			params:  map[string]any{"id": 1},
			wantErr: true,
		},
		{
			name:    "extra params ok",
			query:   "SELECT * FROM users WHERE id = :id",
			params:  map[string]any{"id": 1, "extra": "ignored"},
			wantErr: false,
		},
		{
			name:    "no params needed",
			query:   "SELECT * FROM users",
			params:  map[string]any{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := sqlexec.ValidateParams(tt.query, tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Helper function to create an executor via the registry
func newSQLiteExecutor(ctx context.Context, step core.Step) (executor.Executor, error) {
	return executor.NewExecutor(ctx, step)
}

func TestSQLiteExecutor_InMemory(t *testing.T) {
	ctx := context.Background()

	// Create a step with SQLite config
	step := core.Step{
		Name: "test-sqlite",
		ExecutorConfig: core.ExecutorConfig{
			Type: "sqlite",
			Config: map[string]any{
				"dsn": ":memory:",
			},
		},
		Script: `
			CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT);
			INSERT INTO test (name) VALUES ('Alice'), ('Bob');
			SELECT * FROM test;
		`,
	}

	exec, err := newSQLiteExecutor(ctx, step)
	require.NoError(t, err)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exec.SetStdout(&stdout)
	exec.SetStderr(&stderr)

	err = exec.Run(ctx)
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")
}

func TestSQLiteExecutor_FileDB(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	// Create a step with SQLite file database
	step := core.Step{
		Name: "test-sqlite-file",
		ExecutorConfig: core.ExecutorConfig{
			Type: "sqlite",
			Config: map[string]any{
				"dsn": "file:" + dbPath + "?mode=rwc",
			},
		},
		Script: `
			CREATE TABLE IF NOT EXISTS counter (id INTEGER PRIMARY KEY, value INTEGER);
			INSERT INTO counter (value) VALUES (42);
			SELECT value FROM counter;
		`,
	}

	exec, err := newSQLiteExecutor(ctx, step)
	require.NoError(t, err)

	var stdout bytes.Buffer
	exec.SetStdout(&stdout)

	err = exec.Run(ctx)
	require.NoError(t, err)

	output := stdout.String()
	assert.Contains(t, output, "42")

	// Verify file was created
	_, err = os.Stat(dbPath)
	assert.NoError(t, err)
}

func TestSQLiteExecutor_Transaction(t *testing.T) {
	ctx := context.Background()

	step := core.Step{
		Name: "test-transaction",
		ExecutorConfig: core.ExecutorConfig{
			Type: "sqlite",
			Config: map[string]any{
				"dsn":         ":memory:",
				"transaction": true,
			},
		},
		Script: `
			CREATE TABLE accounts (id INTEGER PRIMARY KEY, balance INTEGER);
			INSERT INTO accounts (balance) VALUES (100), (200);
			UPDATE accounts SET balance = balance - 50 WHERE id = 1;
			UPDATE accounts SET balance = balance + 50 WHERE id = 2;
			SELECT SUM(balance) as total FROM accounts;
		`,
	}

	exec, err := newSQLiteExecutor(ctx, step)
	require.NoError(t, err)

	var stdout bytes.Buffer
	exec.SetStdout(&stdout)

	err = exec.Run(ctx)
	require.NoError(t, err)

	// Total should be 300 (100 + 200, unchanged by transfer)
	output := stdout.String()
	assert.Contains(t, output, "300")
}

func TestSQLiteExecutor_OutputFormats(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		outputFormat string
		headers      bool
		contains     []string
		notContains  []string
	}{
		{
			name:         "jsonl",
			outputFormat: "jsonl",
			contains:     []string{`"id":1`, `"name":"Alice"`},
		},
		{
			name:         "csv with headers",
			outputFormat: "csv",
			headers:      true,
			contains:     []string{"id,name", "1,Alice"},
		},
		{
			name:         "csv without headers",
			outputFormat: "csv",
			headers:      false,
			contains:     []string{"1,Alice"},
			notContains:  []string{"id,name"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := core.Step{
				Name: "test-format",
				ExecutorConfig: core.ExecutorConfig{
					Type: "sqlite",
					Config: map[string]any{
						"dsn":          ":memory:",
						"outputFormat": tt.outputFormat,
						"headers":      tt.headers,
					},
				},
				Script: `
					CREATE TABLE test (id INTEGER, name TEXT);
					INSERT INTO test VALUES (1, 'Alice');
					SELECT * FROM test;
				`,
			}

			exec, err := newSQLiteExecutor(ctx, step)
			require.NoError(t, err)

			var stdout bytes.Buffer
			exec.SetStdout(&stdout)

			err = exec.Run(ctx)
			require.NoError(t, err)

			output := stdout.String()
			for _, s := range tt.contains {
				assert.Contains(t, output, s)
			}
			for _, s := range tt.notContains {
				assert.NotContains(t, output, s)
			}
		})
	}
}

func TestSQLiteExecutor_MaxRows(t *testing.T) {
	ctx := context.Background()

	step := core.Step{
		Name: "test-maxrows",
		ExecutorConfig: core.ExecutorConfig{
			Type: "sqlite",
			Config: map[string]any{
				"dsn":     ":memory:",
				"maxRows": 2,
			},
		},
		Script: `
			CREATE TABLE numbers (n INTEGER);
			INSERT INTO numbers VALUES (1), (2), (3), (4), (5);
			SELECT n FROM numbers ORDER BY n;
		`,
	}

	exec, err := newSQLiteExecutor(ctx, step)
	require.NoError(t, err)

	var stdout bytes.Buffer
	exec.SetStdout(&stdout)

	err = exec.Run(ctx)
	require.NoError(t, err)

	output := stdout.String()
	// Should only have 2 rows
	assert.Contains(t, output, `"n":1`)
	assert.Contains(t, output, `"n":2`)
	assert.NotContains(t, output, `"n":3`)
}
