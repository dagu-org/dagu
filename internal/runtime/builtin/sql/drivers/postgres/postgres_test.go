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

func TestPostgresDriver_BuildInsertQuery(t *testing.T) {
	driver := &PostgresDriver{}

	tests := []struct {
		name           string
		table          string
		columns        []string
		rowCount       int
		onConflict     string
		conflictTarget string
		updateColumns  []string
		want           string
	}{
		{
			name:       "single row",
			table:      "users",
			columns:    []string{"name", "age"},
			rowCount:   1,
			onConflict: "error",
			want:       `INSERT INTO "users" ("name", "age") VALUES ($1, $2)`,
		},
		{
			name:       "multiple rows",
			table:      "users",
			columns:    []string{"name", "age"},
			rowCount:   3,
			onConflict: "error",
			want:       `INSERT INTO "users" ("name", "age") VALUES ($1, $2), ($3, $4), ($5, $6)`,
		},
		{
			name:       "with ignore",
			table:      "users",
			columns:    []string{"id", "name"},
			rowCount:   2,
			onConflict: "ignore",
			want:       `INSERT INTO "users" ("id", "name") VALUES ($1, $2), ($3, $4) ON CONFLICT DO NOTHING`,
		},
		{
			name:       "with replace no target",
			table:      "users",
			columns:    []string{"id", "name"},
			rowCount:   1,
			onConflict: "replace",
			want:       `INSERT INTO "users" ("id", "name") VALUES ($1, $2) ON CONFLICT DO NOTHING`,
		},
		{
			name:           "with replace and conflict target",
			table:          "users",
			columns:        []string{"id", "name", "email"},
			rowCount:       1,
			onConflict:     "replace",
			conflictTarget: "id",
			want:           `INSERT INTO "users" ("id", "name", "email") VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET "name" = EXCLUDED."name", "email" = EXCLUDED."email"`,
		},
		{
			name:           "with replace and composite conflict target",
			table:          "user_orgs",
			columns:        []string{"user_id", "org_id", "role"},
			rowCount:       1,
			onConflict:     "replace",
			conflictTarget: "user_id, org_id",
			want:           `INSERT INTO "user_orgs" ("user_id", "org_id", "role") VALUES ($1, $2, $3) ON CONFLICT (user_id, org_id) DO UPDATE SET "role" = EXCLUDED."role"`,
		},
		{
			name:           "with replace and explicit update columns",
			table:          "users",
			columns:        []string{"id", "name", "email", "updated_at"},
			rowCount:       1,
			onConflict:     "replace",
			conflictTarget: "id",
			updateColumns:  []string{"name", "updated_at"},
			want:           `INSERT INTO "users" ("id", "name", "email", "updated_at") VALUES ($1, $2, $3, $4) ON CONFLICT (id) DO UPDATE SET "name" = EXCLUDED."name", "updated_at" = EXCLUDED."updated_at"`,
		},
		{
			name:       "single column",
			table:      "items",
			columns:    []string{"value"},
			rowCount:   2,
			onConflict: "error",
			want:       `INSERT INTO "items" ("value") VALUES ($1), ($2)`,
		},
		{
			name:       "reserved word table name",
			table:      "order",
			columns:    []string{"select", "from"},
			rowCount:   1,
			onConflict: "error",
			want:       `INSERT INTO "order" ("select", "from") VALUES ($1, $2)`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driver.BuildInsertQuery(tt.table, tt.columns, tt.rowCount, tt.onConflict, tt.conflictTarget, tt.updateColumns)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgresDriver_QuoteIdentifier(t *testing.T) {
	driver := &PostgresDriver{}

	tests := []struct {
		name string
		want string
	}{
		{"users", `"users"`},
		{"order", `"order"`},                     // Reserved word
		{`table"name`, `"table""name"`},          // Contains quote
		{"CamelCase", `"CamelCase"`},             // Preserves case
		{"with spaces", `"with spaces"`},         // Contains spaces
		{"special!@#chars", `"special!@#chars"`}, // Special characters
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := driver.QuoteIdentifier(tt.name)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgresDriver_ConvertNamedParams(t *testing.T) {
	driver := &PostgresDriver{}

	tests := []struct {
		name       string
		query      string
		params     map[string]any
		wantQuery  string
		wantParams []any
		wantErr    bool
	}{
		{
			name:       "single parameter",
			query:      "SELECT * FROM users WHERE id = :id",
			params:     map[string]any{"id": 123},
			wantQuery:  "SELECT * FROM users WHERE id = $1",
			wantParams: []any{123},
			wantErr:    false,
		},
		{
			name:       "multiple parameters",
			query:      "SELECT * FROM users WHERE name = :name AND status = :status",
			params:     map[string]any{"name": "Alice", "status": "active"},
			wantQuery:  "SELECT * FROM users WHERE name = $1 AND status = $2",
			wantParams: []any{"Alice", "active"},
			wantErr:    false,
		},
		{
			name:       "repeated parameter",
			query:      "SELECT * FROM users WHERE id = :id OR parent_id = :id",
			params:     map[string]any{"id": 123},
			wantQuery:  "SELECT * FROM users WHERE id = $1 OR parent_id = $1",
			wantParams: []any{123},
			wantErr:    false,
		},
		{
			name:       "no parameters in query",
			query:      "SELECT * FROM users",
			params:     map[string]any{"id": 123},
			wantQuery:  "SELECT * FROM users",
			wantParams: nil,
			wantErr:    false,
		},
		{
			name:       "missing parameter",
			query:      "SELECT * FROM users WHERE id = :id",
			params:     map[string]any{"other": 123},
			wantQuery:  "",
			wantParams: nil,
			wantErr:    true,
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
