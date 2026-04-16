// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCustomStepInput_DefersRuntimeExpressionLeaves(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		schema    map[string]any
		input     map[string]any
		assertErr assert.ErrorAssertionFunc
	}{
		{
			name: "IntegerWholeRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"count": map[string]any{"type": "integer"},
			}, "count"),
			input:     map[string]any{"count": "${COUNT}"},
			assertErr: assert.NoError,
		},
		{
			name: "BooleanWholeRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"enabled": map[string]any{"type": "boolean"},
			}, "enabled"),
			input:     map[string]any{"enabled": "$ENABLED"},
			assertErr: assert.NoError,
		},
		{
			name: "EnumWholeRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"mode": map[string]any{"enum": []any{"fast", "slow"}},
			}, "mode"),
			input:     map[string]any{"mode": "${MODE}"},
			assertErr: assert.NoError,
		},
		{
			name: "StringEmbeddedRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"message": map[string]any{"type": "string"},
			}, "message"),
			input:     map[string]any{"message": "hello-${NAME}"},
			assertErr: assert.NoError,
		},
		{
			name: "NestedIntegerRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"limits": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"count": map[string]any{"type": "integer"},
					},
					"required": []any{"count"},
				},
			}, "limits"),
			input:     map[string]any{"limits": map[string]any{"count": "${COUNT}"}},
			assertErr: assert.NoError,
		},
		{
			name: "ArrayIntegerRuntimeExpression",
			schema: objectInputSchema(map[string]any{
				"counts": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "integer"},
				},
			}, "counts"),
			input:     map[string]any{"counts": []any{"${COUNT}"}},
			assertErr: assert.NoError,
		},
		{
			name: "IntegerEmbeddedRuntimeExpressionRejected",
			schema: objectInputSchema(map[string]any{
				"count": map[string]any{"type": "integer"},
			}, "count"),
			input:     map[string]any{"count": "count-${COUNT}"},
			assertErr: assert.Error,
		},
		{
			name: "NonRuntimeInvalidStillRejected",
			schema: objectInputSchema(map[string]any{
				"count": map[string]any{"type": "integer"},
			}, "count"),
			input:     map[string]any{"count": "abc"},
			assertErr: assert.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			schema := mustResolveCustomStepInputSchema(t, tt.schema)
			_, err := validateCustomStepInput("test", schema, tt.input)
			tt.assertErr(t, err)
		})
	}
}

func objectInputSchema(properties map[string]any, required ...string) map[string]any {
	requiredValues := make([]any, 0, len(required))
	for _, name := range required {
		requiredValues = append(requiredValues, name)
	}
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             requiredValues,
		"properties":           properties,
	}
}

func mustResolveCustomStepInputSchema(t *testing.T, schema map[string]any) *jsonschema.Resolved {
	t.Helper()

	resolved, err := resolveCustomStepTypeInputSchema("test", schema)
	require.NoError(t, err)
	return resolved
}
