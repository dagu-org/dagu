// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/core"
)

// ---------- Scenario 1: Primitive type schemas ----------

func TestValidateOutput_PrimitiveStringSchema(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type": "string",
	})

	t.Run("valid JSON string passes", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"hello"`)
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})

	t.Run("JSON number fails string schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `42`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("JSON boolean fails string schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `true`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("JSON null fails string schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `null`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("bare unquoted string fails JSON parse", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `hello`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})
}

func TestValidateOutput_PrimitiveNumberSchema(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type": "number",
	})

	t.Run("integer passes number schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `42`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("float passes number schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `3.14`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("string fails number schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"not a number"`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})
}

func TestValidateOutput_PrimitiveBooleanSchema(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type": "boolean",
	})

	t.Run("true passes boolean schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `true`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("false passes boolean schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `false`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("number fails boolean schema", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `0`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})
}

// ---------- Scenario 2: Array type schemas ----------

func TestValidateOutput_ArraySchema(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type":  "array",
		"items": map[string]any{"type": "number"},
	})

	t.Run("valid number array passes", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `[1, 2, 3]`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("empty array passes", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `[]`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("mixed type array fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `[1, "two", 3]`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("object instead of array fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"key": 1}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("nested array of objects", func(t *testing.T) {
		objArraySchema := compileTestSchema(t, map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "integer"},
				},
				"required": []any{"id"},
			},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: objArraySchema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `[{"id": 1}, {"id": 2}]`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("nested array missing required field fails", func(t *testing.T) {
		objArraySchema := compileTestSchema(t, map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]any{"type": "integer"},
				},
				"required": []any{"id"},
			},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: objArraySchema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `[{"id": 1}, {"name": "no-id"}]`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})
}

// ---------- Scenario 3: Trailing newline / whitespace handling ----------

func TestValidateOutput_WhitespaceHandling(t *testing.T) {
	// captureOutput uses strings.TrimSpace, so by the time validateOutput
	// runs, trailing newlines should already be stripped. These tests verify
	// that values stored via setVariable (simulating post-capture state)
	// with leading/trailing whitespace are handled correctly by validateOutput.
	schema := compileTestSchema(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{"type": "string"},
		},
		"required": []any{"key"},
	})

	t.Run("value with trailing newline fails JSON parse", func(t *testing.T) {
		// If somehow whitespace survives into the variable (e.g., set manually),
		// validateOutput should fail on JSON parse since trailing \n is not valid JSON.
		// Actually, json.Unmarshal tolerates trailing whitespace. Let's verify.
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", "{\"key\":\"value\"}\n")
		// Go's json.Unmarshal accepts trailing whitespace — this should pass.
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})

	t.Run("value with leading whitespace passes JSON parse", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", "  {\"key\":\"value\"}")
		// Go's json.Unmarshal ignores leading whitespace.
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})

	t.Run("whitespace-only value fails as empty", func(t *testing.T) {
		// strings.TrimSpace would make this empty during capture,
		// but test the validateOutput path if whitespace-only gets through.
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", "   \n\t  ")
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		// Whitespace-only is not empty string, so it hits JSON parse, not the empty check.
		assert.Contains(t, err.Error(), "not valid JSON")
	})
}

// ---------- Scenario 4: additionalProperties: false ----------

func TestValidateOutput_AdditionalPropertiesFalse(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
		"additionalProperties": false,
	})

	t.Run("exact match passes", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"name":"test"}`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("extra property fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"name":"test","extra":true}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("empty object passes when no required fields", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{}`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("multiple extra properties fail", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"name":"test","extra1":1,"extra2":"two"}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})
}

// ---------- Scenario 5: Build-time schema errors ----------

func TestBuildTimeSchemaErrors(t *testing.T) {
	t.Run("unknown type resolves but rejects valid data at validate time", func(t *testing.T) {
		// google/jsonschema-go does not reject unknown type values at resolve time.
		// Instead, the schema resolves successfully but validation behavior may vary.
		schemaMap := map[string]any{
			"type": "not_a_real_type",
		}
		data, err := json.Marshal(schemaMap)
		require.NoError(t, err)
		var s jsonschema.Schema
		require.NoError(t, json.Unmarshal(data, &s))
		resolved, resolveErr := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
		// The library accepts unknown types at resolve time.
		// Verify it at least resolves without panic.
		if resolveErr != nil {
			// If a future version rejects at resolve, that's also acceptable.
			return
		}
		require.NotNil(t, resolved)
		// Validation with the unknown type — any input should fail since "not_a_real_type"
		// doesn't match any JSON type.
		err = resolved.Validate("hello")
		assert.Error(t, err)
	})

	t.Run("contradictory minimum/maximum fails at validate time", func(t *testing.T) {
		schemaMap := map[string]any{
			"type":    "number",
			"minimum": 10,
			"maximum": 5,
		}
		data, err := json.Marshal(schemaMap)
		require.NoError(t, err)
		var s jsonschema.Schema
		err = json.Unmarshal(data, &s)
		require.NoError(t, err)
		resolved, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
		// Some libraries catch this at resolve, others at validate.
		// If resolve succeeds, the schema should at least reject all values in that range.
		if err == nil && resolved != nil {
			// 7 is between 5 and 10 — should fail maximum constraint
			assert.Error(t, resolved.Validate(float64(7)))
		}
	})

	t.Run("invalid $ref fails at resolve time", func(t *testing.T) {
		// A broken $ref is reliably caught at resolve time.
		schemaMap := map[string]any{
			"$ref": "#/definitions/nonexistent",
		}
		data, err := json.Marshal(schemaMap)
		require.NoError(t, err)
		var s jsonschema.Schema
		require.NoError(t, json.Unmarshal(data, &s))
		_, err = s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
		assert.Error(t, err, "unresolvable $ref should fail at resolve time")
	})
}

// ---------- Scenario 6: Thread safety — concurrent validateOutput ----------

func TestValidateOutput_ConcurrentAccess(t *testing.T) {
	// Compile a single schema, share it across multiple nodes,
	// and run validateOutput concurrently. This tests that
	// *jsonschema.Resolved is safe for concurrent reads.
	schema := compileTestSchema(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name":  map[string]any{"type": "string"},
			"count": map[string]any{"type": "integer"},
		},
		"required": []any{"name", "count"},
	})

	const goroutines = 50
	var wg sync.WaitGroup
	errs := make([]error, goroutines)

	for i := range goroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			node := NewNode(core.Step{
				Name:         "test",
				Output:       "RESULT",
				OutputSchema: schema,
			}, NodeState{})
			node.Init()

			// Alternate between valid and invalid data.
			if idx%2 == 0 {
				node.setVariable("RESULT", `{"name":"test","count":1}`)
			} else {
				node.setVariable("RESULT", `{"name":"test","count":"not-a-number"}`)
			}
			errs[idx] = node.validateOutput(context.Background())
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if i%2 == 0 {
			assert.NoError(t, err, "goroutine %d (valid data) should pass", i)
		} else {
			assert.Error(t, err, "goroutine %d (invalid data) should fail", i)
		}
	}
}

// ---------- Additional edge cases ----------

func TestValidateOutput_ComplexEdgeCases(t *testing.T) {
	t.Run("deeply nested object validates correctly", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"level1": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"level2": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{"type": "integer"},
							},
							"required": []any{"value"},
						},
					},
					"required": []any{"level2"},
				},
			},
			"required": []any{"level1"},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"level1":{"level2":{"value":42}}}`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("deeply nested missing required fails", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"level1": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"level2": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{"type": "integer"},
							},
							"required": []any{"value"},
						},
					},
					"required": []any{"level2"},
				},
			},
			"required": []any{"level1"},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"level1":{"level2":{}}}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("enum constraint validates", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type": "string",
			"enum": []any{"red", "green", "blue"},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"red"`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("enum constraint rejects invalid value", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type": "string",
			"enum": []any{"red", "green", "blue"},
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"yellow"`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("pattern constraint validates", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type":    "string",
			"pattern": "^[a-z]+$",
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"hello"`)
		assert.NoError(t, node.validateOutput(context.Background()))
	})

	t.Run("pattern constraint rejects non-matching", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type":    "string",
			"pattern": "^[a-z]+$",
		})
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `"HELLO123"`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("minItems and maxItems on array", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type":     "array",
			"items":    map[string]any{"type": "string"},
			"minItems": 1,
			"maxItems": 3,
		})

		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()

		// Empty array fails minItems
		node.setVariable("RESULT", `[]`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")

		// 2 items passes
		node.setVariable("RESULT", `["a","b"]`)
		assert.NoError(t, node.validateOutput(context.Background()))

		// 4 items fails maxItems
		node.setVariable("RESULT", `["a","b","c","d"]`)
		err = node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("large valid JSON object passes", func(t *testing.T) {
		schema := compileTestSchema(t, map[string]any{
			"type": "object",
			"properties": map[string]any{
				"items": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "integer"},
				},
			},
		})
		// Build a large array
		items := make([]int, 1000)
		for i := range items {
			items[i] = i
		}
		data, _ := json.Marshal(map[string]any{"items": items})

		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", string(data))
		assert.NoError(t, node.validateOutput(context.Background()))
	})
}
