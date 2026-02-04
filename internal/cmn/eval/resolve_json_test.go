package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveJSONPath_ParseError(t *testing.T) {
	ctx := context.Background()
	_, ok := resolveJSONPath(ctx, "VAR", `{"a":1}`, ".[invalid")
	assert.False(t, ok)
}

func TestResolveJSONPath_NoResult(t *testing.T) {
	ctx := context.Background()
	_, ok := resolveJSONPath(ctx, "VAR", `{"a":1}`, "empty")
	assert.False(t, ok)
}

func TestResolveJSONPath_ErrorResult(t *testing.T) {
	ctx := context.Background()
	_, ok := resolveJSONPath(ctx, "VAR", `"not_an_object"`, ".bar.baz")
	assert.False(t, ok)
}

func TestExpandReferences_ComplexJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		want    string
	}{
		{
			name:  "ArrayAccess",
			input: "${DATA.items.[1].name}",
			dataMap: map[string]string{
				"DATA": `{"items": [{"name": "first"}, {"name": "second"}, {"name": "third"}]}`,
			},
			want: "second",
		},
		{
			name:  "BooleanValue",
			input: "${CONFIG.enabled}",
			dataMap: map[string]string{
				"CONFIG": `{"enabled": true}`,
			},
			want: "true",
		},
		{
			name:  "NumberValue",
			input: "${CONFIG.port}",
			dataMap: map[string]string{
				"CONFIG": `{"port": 8080}`,
			},
			want: "8080",
		},
		{
			name:  "NullValue",
			input: "${CONFIG.optional}",
			dataMap: map[string]string{
				"CONFIG": `{"optional": null}`,
			},
			want: "<nil>",
		},
		{
			name:  "DeeplyNested",
			input: "${DATA.level1.level2.level3.value}",
			dataMap: map[string]string{
				"DATA": `{"level1": {"level2": {"level3": {"value": "deep"}}}}`,
			},
			want: "deep",
		},
		{
			name:  "ArrayOfObjects",
			input: "${USERS.[0].email}",
			dataMap: map[string]string{
				"USERS": `[{"name": "Alice", "email": "alice@example.com"}, {"name": "Bob", "email": "bob@example.com"}]`,
			},
			want: "alice@example.com",
		},
		{
			name:  "SpecialCharactersInJSON",
			input: "${DATA.message}",
			dataMap: map[string]string{
				"DATA": `{"message": "Hello \"World\" with 'quotes'"}`,
			},
			want: `Hello "World" with 'quotes'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferences(ctx, tt.input, tt.dataMap)
			assert.Equal(t, tt.want, got)
		})
	}
}
