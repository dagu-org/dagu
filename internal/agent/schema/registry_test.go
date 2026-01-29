package schema

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *Registry {
	return &Registry{schemas: make(map[string]map[string]any)}
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON schema", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		validSchema := []byte(`{"type": "object", "properties": {"name": {"type": "string"}}}`)
		err := r.Register("test", validSchema)
		assert.NoError(t, err)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		invalidJSON := []byte(`{invalid}`)
		err := r.Register("invalid", invalidJSON)
		assert.Error(t, err)
	})
}

func TestRegistry_Navigate_RootLevel(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"description": "Test schema",
		"properties": {
			"name": {"type": "string", "description": "The name field"},
			"count": {"type": "integer", "description": "A counter"}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "")
	require.NoError(t, err)

	assert.Contains(t, result, "name")
	assert.Contains(t, result, "count")
}

func TestRegistry_Navigate_Path(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"config": {
				"type": "object",
				"description": "Configuration object",
				"properties": {
					"host": {"type": "string", "description": "Hostname"},
					"port": {"type": "integer", "description": "Port number"}
				}
			}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "config")
	require.NoError(t, err)

	assert.Contains(t, result, "host")
	assert.Contains(t, result, "port")
}

func TestRegistry_Navigate_InvalidPath(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	_, err := r.Navigate("test", "nonexistent")
	assert.Error(t, err)
}

func TestRegistry_Navigate_UnknownSchema(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	_, err := r.Navigate("unknown", "")
	assert.Error(t, err)
}

func TestRegistry_Navigate_RefResolution(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"item": {"$ref": "#/definitions/myType"}
		},
		"definitions": {
			"myType": {
				"type": "object",
				"description": "A referenced type",
				"properties": {
					"value": {"type": "string", "description": "The value"}
				}
			}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "item")
	require.NoError(t, err)

	assert.Contains(t, result, "value")
}

func TestRegistry_Navigate_ArrayItems(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"items": {
				"type": "array",
				"description": "List of items",
				"items": {
					"type": "object",
					"properties": {
						"name": {"type": "string", "description": "Item name"},
						"value": {"type": "integer", "description": "Item value"}
					}
				}
			}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "items.name")
	require.NoError(t, err)

	assert.Contains(t, result, "string")
}

func TestRegistry_Navigate_OneOf(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"value": {
				"oneOf": [
					{"type": "string", "description": "String value"},
					{"type": "integer", "description": "Integer value"},
					{"type": "object", "description": "Object value", "properties": {"nested": {"type": "string"}}}
				]
			}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "value")
	require.NoError(t, err)

	assert.Contains(t, result, "oneOf")
	assert.Contains(t, result, "Option")
}

func TestRegistry_Navigate_AllOf(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	schema := []byte(`{
		"type": "object",
		"properties": {
			"combined": {
				"allOf": [
					{"type": "object", "properties": {"field1": {"type": "string"}}},
					{"type": "object", "properties": {"field2": {"type": "integer"}}}
				]
			}
		}
	}`)
	require.NoError(t, r.Register("test", schema))

	result, err := r.Navigate("test", "combined")
	require.NoError(t, err)

	assert.Contains(t, result, "field1")
	assert.Contains(t, result, "field2")
}

func TestRegistry_AvailableSchemas(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	require.NoError(t, r.Register("schema1", []byte(`{"type": "object"}`)))
	require.NoError(t, r.Register("schema2", []byte(`{"type": "object"}`)))

	available := r.AvailableSchemas()
	assert.Len(t, available, 2)
}

func TestRegistry_Navigate_DagSchema(t *testing.T) {
	t.Parallel()

	available := DefaultRegistry.AvailableSchemas()
	if len(available) == 0 {
		t.Skip("No schemas registered (embed may not have run)")
	}

	if !slices.Contains(available, "dag") {
		t.Skip("dag schema not registered")
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		want    []string
	}{
		{
			name:    "root level",
			path:    "",
			wantErr: false,
			want:    []string{"steps", "schedule", "name"},
		},
		{
			name:    "steps field",
			path:    "steps",
			wantErr: false,
			want:    []string{"array"},
		},
		{
			name:    "handlerOn",
			path:    "handlerOn",
			wantErr: false,
			want:    []string{"success", "failure"},
		},
		{
			name:    "invalid path",
			path:    "nonexistent.field",
			wantErr: true,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			result, err := DefaultRegistry.Navigate("dag", tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)

			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("Navigate() result missing %q\nGot: %s", want, result)
				}
			}
		})
	}
}
