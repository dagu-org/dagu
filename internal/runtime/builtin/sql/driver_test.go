package sql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
)

// mockDriver implements Driver interface for testing.
type mockDriver struct {
	name string
}

func (d *mockDriver) Name() string                       { return d.name }
func (d *mockDriver) PlaceholderFormat() string          { return "?" }
func (d *mockDriver) SupportsAdvisoryLock() bool         { return false }
func (d *mockDriver) QuoteIdentifier(name string) string { return QuoteIdentifier(name) }
func (d *mockDriver) Connect(_ context.Context, _ *Config) (*sql.DB, func() error, error) {
	return nil, nil, nil
}
func (d *mockDriver) AcquireAdvisoryLock(_ context.Context, _ *sql.DB, _ string) (func() error, error) {
	return nil, nil
}
func (d *mockDriver) ConvertNamedParams(query string, params map[string]any) (string, []any, error) {
	return query, nil, nil
}
func (d *mockDriver) BuildInsertQuery(table string, columns []string, rowCount int, onConflict, conflictTarget string, updateColumns []string) string {
	return ""
}

func TestQuoteIdentifier(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple", "users", `"users"`},
		{"reserved word", "order", `"order"`},
		{"with double quote", `table"name`, `"table""name"`},
		{"camelCase", "CamelCase", `"CamelCase"`},
		{"with spaces", "with spaces", `"with spaces"`},
		{"special chars", "special!@#chars", `"special!@#chars"`},
		{"empty string", "", `""`},
		{"multiple quotes", `a"b"c`, `"a""b""c"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := QuoteIdentifier(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseConflictTarget(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"single column", "id", []string{"id"}},
		{"with quotes", `"id"`, []string{"id"}},
		{"composite", "(user_id, org_id)", []string{"user_id", "org_id"}},
		{"composite with quotes", `("user_id", "org_id")`, []string{"user_id", "org_id"}},
		{"spaces", "  id  ", []string{"id"}},
		{"composite with spaces", " user_id , org_id ", []string{"user_id", "org_id"}},
		{"empty", "", []string{}},
		{"single parens", "(id)", []string{"id"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ParseConflictTarget(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		slice    []string
		item     string
		expected bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"single match", []string{"a"}, "a", true},
		{"case sensitive", []string{"A"}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := Contains(tt.slice, tt.item)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDriverRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	registry := NewDriverRegistry()

	// Test concurrent registration and retrieval
	done := make(chan bool)
	for range 10 {
		go func() {
			registry.Register(&mockDriver{name: "test"})
			_, _ = registry.Get("test")
			done <- true
		}()
	}

	for range 10 {
		<-done
	}

	driver, ok := registry.Get("test")
	assert.True(t, ok)
	assert.Equal(t, "test", driver.Name())
}
