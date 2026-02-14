package schema

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestRegistry() *Registry {
	return &Registry{schemas: make(map[string]map[string]any)}
}

// registerTestSchema creates a registry with a single schema registered under "test".
func registerTestSchema(t *testing.T, schemaJSON string) *Registry {
	t.Helper()
	r := newTestRegistry()
	require.NoError(t, r.Register("test", []byte(schemaJSON)))
	return r
}

func TestRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("valid JSON schema", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		err := r.Register("test", []byte(`{"type": "object", "properties": {"name": {"type": "string"}}}`))
		assert.NoError(t, err)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		err := r.Register("invalid", []byte(`{invalid}`))
		assert.Error(t, err)
	})

	t.Run("overwrite existing schema", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()

		require.NoError(t, r.Register("test", []byte(`{"type": "object", "properties": {"field1": {"type": "string"}}}`)))
		result1, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result1, "field1")

		require.NoError(t, r.Register("test", []byte(`{"type": "object", "properties": {"field2": {"type": "integer"}}}`)))
		result2, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result2, "field2")
		assert.NotContains(t, result2, "field1")
	})
}

func TestRegistry_Navigate(t *testing.T) {
	t.Parallel()

	t.Run("root level shows all properties", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"description": "Test schema",
			"properties": {
				"name": {"type": "string", "description": "The name field"},
				"count": {"type": "integer", "description": "A counter"}
			}
		}`)

		result, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result, "name")
		assert.Contains(t, result, "count")
	})

	t.Run("nested path shows child properties", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
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

		result, err := r.Navigate("test", "config")
		require.NoError(t, err)
		assert.Contains(t, result, "host")
		assert.Contains(t, result, "port")
	})

	t.Run("empty path shows root description", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"description": "Root schema description",
			"properties": {
				"name": {"type": "string"}
			}
		}`)

		result, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result, "Test Schema Root")
		assert.Contains(t, result, "Root schema description")
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{"type": "object", "properties": {"name": {"type": "string"}}}`)

		_, err := r.Navigate("test", "nonexistent")
		assert.Error(t, err)
	})

	t.Run("unknown schema returns error", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		_, err := r.Navigate("unknown", "")
		assert.Error(t, err)
	})

	t.Run("scalar field with child path returns error", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"scalar": {"type": "string", "description": "A scalar field"}
			}
		}`)

		_, err := r.Navigate("test", "scalar.child")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no properties")
	})

	t.Run("field not found shows available fields", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"existing": {"type": "string"}
			}
		}`)

		_, err := r.Navigate("test", "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
		assert.Contains(t, err.Error(), "existing")
	})
}

func TestRegistry_Navigate_RefResolution(t *testing.T) {
	t.Parallel()

	t.Run("resolves single ref", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
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

		result, err := r.Navigate("test", "item")
		require.NoError(t, err)
		assert.Contains(t, result, "value")
	})

	t.Run("resolves nested refs", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"item": {"$ref": "#/definitions/TypeA"}
			},
			"definitions": {
				"TypeA": {"$ref": "#/definitions/TypeB"},
				"TypeB": {
					"type": "object",
					"properties": {
						"deep": {"type": "string", "description": "Deeply nested"}
					}
				}
			}
		}`)

		result, err := r.Navigate("test", "item")
		require.NoError(t, err)
		assert.Contains(t, result, "deep")
	})

	t.Run("non-existent definition returns original", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"item": {"$ref": "#/definitions/NonExistent"}
			},
			"definitions": {}
		}`)

		result, err := r.Navigate("test", "item")
		require.NoError(t, err)
		assert.Contains(t, result, "ref")
	})

	t.Run("malformed ref returns original", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"item": {"$ref": "SomeType"}
			}
		}`)

		result, err := r.Navigate("test", "item")
		require.NoError(t, err)
		assert.Contains(t, result, "ref")
	})
}

func TestRegistry_Navigate_ArrayItems(t *testing.T) {
	t.Parallel()

	t.Run("navigate into array item property", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
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

		result, err := r.Navigate("test", "items.name")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("array with ref items", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"items": {"$ref": "#/definitions/Item"}
				}
			},
			"definitions": {
				"Item": {
					"type": "object",
					"properties": {
						"id": {"type": "integer"},
						"name": {"type": "string"}
					}
				}
			}
		}`)

		result, err := r.Navigate("test", "items.id")
		require.NoError(t, err)
		assert.Contains(t, result, "integer")
	})
}

func TestRegistry_Navigate_OneOf(t *testing.T) {
	t.Parallel()

	t.Run("shows oneOf options", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
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

		result, err := r.Navigate("test", "value")
		require.NoError(t, err)
		assert.Contains(t, result, "oneOf")
		assert.Contains(t, result, "Option")
	})

	t.Run("navigate into oneOf with ref", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"item": {
					"oneOf": [
						{"type": "string"},
						{"$ref": "#/definitions/ObjectType"}
					]
				}
			},
			"definitions": {
				"ObjectType": {
					"type": "object",
					"properties": {
						"name": {"type": "string"}
					}
				}
			}
		}`)

		result, err := r.Navigate("test", "item.name")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("non-map entries are skipped", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"value": {
					"oneOf": [
						"not_a_map",
						123,
						{"type": "object", "properties": {"name": {"type": "string"}}}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "value.name")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("array without items property is handled", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"value": {
					"oneOf": [
						{"type": "array"},
						{"type": "object", "properties": {"name": {"type": "string"}}}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "value.name")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("no navigable variants returns error", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"value": {
					"oneOf": [
						{"type": "string"},
						{"type": "integer"}
					]
				}
			}
		}`)

		_, err := r.Navigate("test", "value.child")
		assert.Error(t, err)
	})
}

func TestRegistry_Navigate_AllOf(t *testing.T) {
	t.Parallel()

	t.Run("merges properties from all schemas", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
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

		result, err := r.Navigate("test", "combined")
		require.NoError(t, err)
		assert.Contains(t, result, "field1")
		assert.Contains(t, result, "field2")
	})

	t.Run("merges multiple schemas", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"merged": {
					"allOf": [
						{"type": "object", "properties": {"a": {"type": "string"}}},
						{"type": "object", "properties": {"b": {"type": "integer"}}},
						{"type": "object", "properties": {"c": {"type": "boolean"}}}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "merged")
		require.NoError(t, err)
		assert.Contains(t, result, "a")
		assert.Contains(t, result, "b")
		assert.Contains(t, result, "c")
	})

	t.Run("resolves refs within allOf", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"combined": {
					"allOf": [
						{"$ref": "#/definitions/Base"},
						{"type": "object", "properties": {"extra": {"type": "string"}}}
					]
				}
			},
			"definitions": {
				"Base": {
					"type": "object",
					"properties": {
						"base_field": {"type": "string"}
					}
				}
			}
		}`)

		result, err := r.Navigate("test", "combined")
		require.NoError(t, err)
		assert.Contains(t, result, "base_field")
		assert.Contains(t, result, "extra")
	})
}

func TestRegistry_AvailableSchemas(t *testing.T) {
	t.Parallel()

	t.Run("returns registered schema names", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		require.NoError(t, r.Register("schema1", []byte(`{"type": "object"}`)))
		require.NoError(t, r.Register("schema2", []byte(`{"type": "object"}`)))

		available := r.AvailableSchemas()
		assert.Len(t, available, 2)
	})

	t.Run("empty registry returns empty slice", func(t *testing.T) {
		t.Parallel()
		r := newTestRegistry()
		assert.Empty(t, r.AvailableSchemas())
	})
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
			name:    "handler_on",
			path:    "handler_on",
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

func TestGetType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node map[string]any
		want string
	}{
		{
			name: "explicit type string",
			node: map[string]any{"type": "string"},
			want: "string",
		},
		{
			name: "explicit type integer",
			node: map[string]any{"type": "integer"},
			want: "integer",
		},
		{
			name: "explicit type object",
			node: map[string]any{"type": "object"},
			want: "object",
		},
		{
			name: "oneOf",
			node: map[string]any{"oneOf": []any{map[string]any{"type": "string"}}},
			want: "oneOf",
		},
		{
			name: "anyOf",
			node: map[string]any{"anyOf": []any{map[string]any{"type": "string"}}},
			want: "anyOf",
		},
		{
			name: "allOf",
			node: map[string]any{"allOf": []any{map[string]any{"type": "string"}}},
			want: "allOf",
		},
		{
			name: "ref",
			node: map[string]any{"$ref": "#/definitions/SomeType"},
			want: "ref",
		},
		{
			name: "object inferred from properties",
			node: map[string]any{"properties": map[string]any{"field": map[string]any{}}},
			want: "object",
		},
		{
			name: "empty node returns unknown",
			node: map[string]any{},
			want: "unknown",
		},
		{
			name: "node with only description returns unknown",
			node: map[string]any{"description": "some description"},
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getType(tt.node)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetRequiredSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		node map[string]any
		want map[string]bool
	}{
		{
			name: "node with required fields",
			node: map[string]any{"required": []any{"field1", "field2"}},
			want: map[string]bool{"field1": true, "field2": true},
		},
		{
			name: "node without required returns nil",
			node: map[string]any{"type": "object"},
			want: nil,
		},
		{
			name: "empty required array",
			node: map[string]any{"required": []any{}},
			want: map[string]bool{},
		},
		{
			name: "non-string items in required array are skipped",
			node: map[string]any{"required": []any{"valid", 123, "also_valid", nil}},
			want: map[string]bool{"valid": true, "also_valid": true},
		},
		{
			name: "required is not array",
			node: map[string]any{"required": "not_an_array"},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getRequiredSet(tt.node)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTruncateDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "short description unchanged",
			input: "Short desc",
			want:  "Short desc",
		},
		{
			name:  "nil returns empty string",
			input: nil,
			want:  "",
		},
		{
			name:  "non-string returns empty string",
			input: 123,
			want:  "",
		},
		{
			name:  "exactly 100 chars unchanged",
			input: strings.Repeat("a", 100),
			want:  strings.Repeat("a", 100),
		},
		{
			name:  "101 chars gets truncated",
			input: strings.Repeat("a", 101),
			want:  strings.Repeat("a", 97) + "...",
		},
		{
			name:  "long description truncated with ellipsis",
			input: strings.Repeat("x", 200),
			want:  strings.Repeat("x", 97) + "...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := truncateDescription(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetDefinitions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema map[string]any
		want   map[string]any
	}{
		{
			name:   "schema without definitions returns empty map",
			schema: map[string]any{"type": "object"},
			want:   map[string]any{},
		},
		{
			name: "schema with definitions returns them",
			schema: map[string]any{
				"type": "object",
				"definitions": map[string]any{
					"MyType": map[string]any{"type": "string"},
				},
			},
			want: map[string]any{
				"MyType": map[string]any{"type": "string"},
			},
		},
		{
			name:   "definitions is not a map returns empty map",
			schema: map[string]any{"definitions": "not_a_map"},
			want:   map[string]any{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := getDefinitions(tt.schema)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizeForNavigation_AnyOf(t *testing.T) {
	t.Parallel()

	t.Run("navigates into anyOf object properties", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"field": {
					"anyOf": [
						{"type": "string"},
						{"type": "object", "properties": {"nested": {"type": "string"}}}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "field.nested")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("navigates into anyOf array items", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"items": {
					"anyOf": [
						{"type": "string"},
						{
							"type": "array",
							"items": {
								"type": "object",
								"properties": {
									"name": {"type": "string"}
								}
							}
						}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "items.name")
		require.NoError(t, err)
		assert.Contains(t, result, "string")
	})

	t.Run("handles max depth safely", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"field": {"type": "string"}
			}
		}`)

		result, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result, "field")
	})
}

func TestFormatNode(t *testing.T) {
	t.Parallel()

	t.Run("anyOf formats as oneOf", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"value": {
					"anyOf": [
						{"type": "string", "description": "String option"},
						{"type": "integer", "description": "Integer option"}
					]
				}
			}
		}`)

		result, err := r.Navigate("test", "value")
		require.NoError(t, err)
		assert.Contains(t, result, "oneOf")
		assert.Contains(t, result, "String option")
		assert.Contains(t, result, "Integer option")
	})

	t.Run("array with object items shows item properties", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"items": {
					"type": "array",
					"description": "List of items",
					"items": {
						"type": "object",
						"properties": {
							"name": {"type": "string"},
							"value": {"type": "integer"}
						}
					}
				}
			}
		}`)

		result, err := r.Navigate("test", "items")
		require.NoError(t, err)
		assert.Contains(t, result, "Items:")
		assert.Contains(t, result, "name")
		assert.Contains(t, result, "value")
	})

	t.Run("array with scalar items shows item type", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"tags": {
					"type": "array",
					"description": "List of tags",
					"items": {"type": "string"}
				}
			}
		}`)

		result, err := r.Navigate("test", "tags")
		require.NoError(t, err)
		assert.Contains(t, result, "Items:")
		assert.Contains(t, result, "Type: string")
	})

	t.Run("enum shows allowed values", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"status": {
					"type": "string",
					"enum": ["pending", "running", "completed", "failed"]
				}
			}
		}`)

		result, err := r.Navigate("test", "status")
		require.NoError(t, err)
		assert.Contains(t, result, "Allowed values:")
		assert.Contains(t, result, "pending")
		assert.Contains(t, result, "running")
		assert.Contains(t, result, "completed")
		assert.Contains(t, result, "failed")
	})

	t.Run("default value is shown", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"properties": {
				"timeout": {
					"type": "integer",
					"default": 30,
					"description": "Timeout in seconds"
				}
			}
		}`)

		result, err := r.Navigate("test", "timeout")
		require.NoError(t, err)
		assert.Contains(t, result, "Default: 30")
	})

	t.Run("required fields are marked", func(t *testing.T) {
		t.Parallel()
		r := registerTestSchema(t, `{
			"type": "object",
			"required": ["name"],
			"properties": {
				"name": {"type": "string", "description": "Required field"},
				"optional": {"type": "string", "description": "Optional field"}
			}
		}`)

		result, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result, "required")
	})

	t.Run("non-map property is skipped", func(t *testing.T) {
		t.Parallel()
		r := &Registry{schemas: make(map[string]map[string]any)}
		r.schemas["test"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"valid":   map[string]any{"type": "string"},
				"invalid": "not_a_map",
			},
		}

		result, err := r.Navigate("test", "")
		require.NoError(t, err)
		assert.Contains(t, result, "valid")
	})

	t.Run("oneOf with non-map option is handled", func(t *testing.T) {
		t.Parallel()
		r := &Registry{schemas: make(map[string]map[string]any)}
		r.schemas["test"] = map[string]any{
			"type": "object",
			"properties": map[string]any{
				"value": map[string]any{
					"oneOf": []any{
						"string_not_map",
						map[string]any{"type": "string", "description": "Valid option"},
					},
				},
			},
		}

		result, err := r.Navigate("test", "value")
		require.NoError(t, err)
		assert.Contains(t, result, "Valid option")
	})
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	r := newTestRegistry()
	const numGoroutines = 10
	const numOps = 100

	// Pre-register a schema to navigate
	baseSchema := []byte(`{"type": "object", "properties": {"field": {"type": "string"}}}`)
	require.NoError(t, r.Register("base", baseSchema))

	var wg sync.WaitGroup
	errCh := make(chan error, numGoroutines*numOps)

	// Concurrent registers
	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numOps {
				schema := []byte(`{"type": "object", "properties": {"prop": {"type": "string"}}}`)
				if err := r.Register(fmt.Sprintf("schema_%d_%d", id, j), schema); err != nil {
					errCh <- err
				}
			}
		}(i)
	}

	// Concurrent navigates
	for range numGoroutines {
		wg.Go(func() {
			for range numOps {
				if _, err := r.Navigate("base", ""); err != nil {
					errCh <- err
				}
			}
		})
	}

	// Concurrent AvailableSchemas
	for range numGoroutines {
		wg.Go(func() {
			for range numOps {
				_ = r.AvailableSchemas()
			}
		})
	}

	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "concurrent access should not produce errors")
}
