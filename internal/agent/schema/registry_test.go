package schema

import (
	"strings"
	"testing"
)

func TestRegistry_Register(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}

	// Valid JSON schema
	validSchema := []byte(`{"type": "object", "properties": {"name": {"type": "string"}}}`)
	err := r.Register("test", validSchema)
	if err != nil {
		t.Errorf("Register() error = %v, want nil", err)
	}

	// Invalid JSON
	invalidJSON := []byte(`{invalid}`)
	err = r.Register("invalid", invalidJSON)
	if err == nil {
		t.Error("Register() with invalid JSON should return error")
	}
}

func TestRegistry_Navigate_RootLevel(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
	schema := []byte(`{
		"type": "object",
		"description": "Test schema",
		"properties": {
			"name": {"type": "string", "description": "The name field"},
			"count": {"type": "integer", "description": "A counter"}
		}
	}`)
	_ = r.Register("test", schema)

	result, err := r.Navigate("test", "")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	// Should contain properties
	if !strings.Contains(result, "name") {
		t.Error("Navigate() result should contain 'name' property")
	}
	if !strings.Contains(result, "count") {
		t.Error("Navigate() result should contain 'count' property")
	}
}

func TestRegistry_Navigate_Path(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
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
	_ = r.Register("test", schema)

	result, err := r.Navigate("test", "config")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	if !strings.Contains(result, "host") {
		t.Error("Navigate() result should contain 'host' property")
	}
	if !strings.Contains(result, "port") {
		t.Error("Navigate() result should contain 'port' property")
	}
}

func TestRegistry_Navigate_InvalidPath(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
	schema := []byte(`{
		"type": "object",
		"properties": {
			"name": {"type": "string"}
		}
	}`)
	_ = r.Register("test", schema)

	_, err := r.Navigate("test", "nonexistent")
	if err == nil {
		t.Error("Navigate() with invalid path should return error")
	}
}

func TestRegistry_Navigate_UnknownSchema(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}

	_, err := r.Navigate("unknown", "")
	if err == nil {
		t.Error("Navigate() with unknown schema should return error")
	}
}

func TestRegistry_Navigate_RefResolution(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
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
	_ = r.Register("test", schema)

	result, err := r.Navigate("test", "item")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	if !strings.Contains(result, "value") {
		t.Error("Navigate() should resolve $ref and show 'value' property")
	}
}

func TestRegistry_Navigate_ArrayItems(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
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
	_ = r.Register("test", schema)

	// Navigate into array items
	result, err := r.Navigate("test", "items.name")
	if err != nil {
		t.Fatalf("Navigate() into array items error = %v", err)
	}

	if !strings.Contains(result, "string") {
		t.Error("Navigate() into array items should show item property type")
	}
}

func TestRegistry_Navigate_OneOf(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
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
	_ = r.Register("test", schema)

	result, err := r.Navigate("test", "value")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	if !strings.Contains(result, "oneOf") || !strings.Contains(result, "Option") {
		t.Error("Navigate() should show oneOf options")
	}
}

func TestRegistry_Navigate_AllOf(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
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
	_ = r.Register("test", schema)

	result, err := r.Navigate("test", "combined")
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	// Should show merged properties
	if !strings.Contains(result, "field1") || !strings.Contains(result, "field2") {
		t.Error("Navigate() should merge allOf properties")
	}
}

func TestRegistry_AvailableSchemas(t *testing.T) {
	r := &Registry{schemas: make(map[string]map[string]any)}
	_ = r.Register("schema1", []byte(`{"type": "object"}`))
	_ = r.Register("schema2", []byte(`{"type": "object"}`))

	available := r.AvailableSchemas()
	if len(available) != 2 {
		t.Errorf("AvailableSchemas() = %d schemas, want 2", len(available))
	}
}

// Test with real DAG schema
func TestRegistry_Navigate_DagSchema(t *testing.T) {
	// DefaultRegistry should have "dag" schema registered via embed.go init()
	available := DefaultRegistry.AvailableSchemas()
	if len(available) == 0 {
		t.Skip("No schemas registered (embed may not have run)")
	}

	hasDAG := false
	for _, s := range available {
		if s == "dag" {
			hasDAG = true
			break
		}
	}
	if !hasDAG {
		t.Skip("dag schema not registered")
	}

	tests := []struct {
		name    string
		path    string
		wantErr bool
		want    []string // strings that should appear in result
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
			result, err := DefaultRegistry.Navigate("dag", tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Navigate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			for _, want := range tt.want {
				if !strings.Contains(result, want) {
					t.Errorf("Navigate() result missing %q\nGot: %s", want, result)
				}
			}
		})
	}
}
