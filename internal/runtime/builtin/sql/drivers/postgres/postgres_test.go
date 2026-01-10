package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPostgresDriver_Name(t *testing.T) {
	driver := &PostgresDriver{}
	assert.Equal(t, "postgres", driver.Name())
}

func TestPostgresDriver_PlaceholderFormat(t *testing.T) {
	driver := &PostgresDriver{}
	assert.Equal(t, "$", driver.PlaceholderFormat())
}

func TestPostgresDriver_SupportsAdvisoryLock(t *testing.T) {
	driver := &PostgresDriver{}
	assert.True(t, driver.SupportsAdvisoryLock())
}

func TestPostgresDriver_ConvertNamedParams(t *testing.T) {
	driver := &PostgresDriver{}

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
			wantQuery:   "SELECT * FROM users WHERE id = $1",
			wantParams:  []any{123},
			wantErr:     false,
		},
		{
			name:        "multiple parameters",
			query:       "SELECT * FROM users WHERE name = :name AND status = :status",
			params:      map[string]any{"name": "Alice", "status": "active"},
			wantQuery:   "SELECT * FROM users WHERE name = $1 AND status = $2",
			wantParams:  []any{"Alice", "active"},
			wantErr:     false,
		},
		{
			name:        "repeated parameter",
			query:       "SELECT * FROM users WHERE id = :id OR parent_id = :id",
			params:      map[string]any{"id": 123},
			wantQuery:   "SELECT * FROM users WHERE id = $1 OR parent_id = $1",
			wantParams:  []any{123},
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

func TestSplitStatements_Basic(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitStatements(tt.script)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitStatements_DollarQuoted(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
		{
			name:   "dollar quoted string",
			script: "SELECT $$ semicolon; here $$",
			want:   []string{"SELECT $$ semicolon; here $$"},
		},
		{
			name:   "tagged dollar quote",
			script: "SELECT $tag$ semicolon; here $tag$",
			want:   []string{"SELECT $tag$ semicolon; here $tag$"},
		},
		{
			name:   "nested dollar quotes",
			script: "SELECT $outer$ contains $inner$ nested $inner$ content $outer$",
			want:   []string{"SELECT $outer$ contains $inner$ nested $inner$ content $outer$"},
		},
		{
			name:   "function with dollar quote",
			script: "CREATE FUNCTION foo() AS $$ SELECT 1; $$ LANGUAGE sql; SELECT 2",
			want:   []string{"CREATE FUNCTION foo() AS $$ SELECT 1; $$ LANGUAGE sql", "SELECT 2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitStatements(tt.script)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestSplitStatements_NestedQuotes(t *testing.T) {
	tests := []struct {
		name   string
		script string
		want   []string
	}{
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
		{
			name:   "mixed quotes",
			script: `SELECT 'single; quotes', "double; quotes"`,
			want:   []string{`SELECT 'single; quotes', "double; quotes"`},
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
		{"TABLE users", true},
		{"VALUES (1, 2), (3, 4)", true},
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
			name:     "connection refused",
			err:      errWithMessage("connection refused"),
			contains: "postgres connection refused",
		},
		{
			name:     "password authentication failed",
			err:      errWithMessage("password authentication failed"),
			contains: "postgres authentication failed",
		},
		{
			name:     "does not exist",
			err:      errWithMessage("table foo does not exist"),
			contains: "postgres object not found",
		},
		{
			name:     "syntax error",
			err:      errWithMessage("syntax error at or near"),
			contains: "postgres syntax error",
		},
		{
			name:     "permission denied",
			err:      errWithMessage("permission denied for table"),
			contains: "postgres permission denied",
		},
		{
			name:     "duplicate key",
			err:      errWithMessage("duplicate key value violates"),
			contains: "postgres unique constraint violation",
		},
		{
			name:     "violates foreign key",
			err:      errWithMessage("violates foreign key constraint"),
			contains: "postgres foreign key violation",
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

func TestExtractErrorCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "error with SQLSTATE",
			err:  errWithMessage("ERROR: table not found (SQLSTATE 42P01)"),
			want: "42P01",
		},
		{
			name: "error without SQLSTATE",
			err:  errWithMessage("some generic error"),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractErrorCode(tt.err)
			assert.Equal(t, tt.want, got)
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
			name: "connection error",
			err:  errWithMessage("connection refused"),
			want: 2,
		},
		{
			name: "SQLSTATE 08xxx - connection exception",
			err:  errWithMessage("ERROR (SQLSTATE 08001)"),
			want: 2,
		},
		{
			name: "SQLSTATE 28xxx - invalid authorization",
			err:  errWithMessage("ERROR (SQLSTATE 28000)"),
			want: 3,
		},
		{
			name: "SQLSTATE 42xxx - syntax error",
			err:  errWithMessage("ERROR (SQLSTATE 42601)"),
			want: 4,
		},
		{
			name: "SQLSTATE 23xxx - constraint violation",
			err:  errWithMessage("ERROR (SQLSTATE 23505)"),
			want: 6,
		},
		{
			name: "SQLSTATE 57xxx - operator intervention",
			err:  errWithMessage("ERROR (SQLSTATE 57014)"),
			want: 7,
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
		name     string
		host     string
		port     int
		user     string
		password string
		database string
		sslmode  string
		want     string
	}{
		{
			name:     "full DSN",
			host:     "localhost",
			port:     5432,
			user:     "testuser",
			password: "testpass",
			database: "testdb",
			sslmode:  "disable",
			want:     "postgres://testuser:testpass@localhost:5432/testdb?sslmode=disable",
		},
		{
			name:     "without password",
			host:     "localhost",
			port:     5432,
			user:     "testuser",
			password: "",
			database: "testdb",
			sslmode:  "disable",
			want:     "postgres://testuser@localhost:5432/testdb?sslmode=disable",
		},
		{
			name:     "without user",
			host:     "localhost",
			port:     5432,
			user:     "",
			password: "",
			database: "testdb",
			sslmode:  "disable",
			want:     "postgres://localhost:5432/testdb?sslmode=disable",
		},
		{
			name:     "without port",
			host:     "localhost",
			port:     0,
			user:     "testuser",
			password: "testpass",
			database: "testdb",
			sslmode:  "disable",
			want:     "postgres://testuser:testpass@localhost/testdb?sslmode=disable",
		},
		{
			name:     "without sslmode",
			host:     "localhost",
			port:     5432,
			user:     "testuser",
			password: "testpass",
			database: "testdb",
			sslmode:  "",
			want:     "postgres://testuser:testpass@localhost:5432/testdb",
		},
		{
			name:     "minimal",
			host:     "localhost",
			port:     0,
			user:     "",
			password: "",
			database: "testdb",
			sslmode:  "",
			want:     "postgres://localhost/testdb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildDSN(tt.host, tt.port, tt.user, tt.password, tt.database, tt.sslmode)
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
