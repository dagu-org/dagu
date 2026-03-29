// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package runtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/core"
)

func compileTestSchema(t *testing.T, schemaMap map[string]any) *jsonschema.Resolved {
	t.Helper()
	data, err := json.Marshal(schemaMap)
	require.NoError(t, err)
	var s jsonschema.Schema
	require.NoError(t, json.Unmarshal(data, &s))
	resolved, err := s.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	require.NoError(t, err)
	return resolved
}

func TestValidateOutput(t *testing.T) {
	schema := compileTestSchema(t, map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary":    map[string]any{"type": "string"},
			"confidence": map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
		},
		"required": []any{"summary", "confidence"},
	})

	t.Run("no schema skips validation", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:   "test",
			Output: "RESULT",
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", "plain text")
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})

	t.Run("valid JSON passes", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"summary":"hello","confidence":0.9}`)
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})

	t.Run("invalid JSON fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"summary":"hello","confidence":2.0}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("non-JSON output fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", "not json at all")
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not valid JSON")
	})

	t.Run("empty output fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		// Do NOT call setVariable - simulates empty capture
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "empty")
	})

	t.Run("missing required field fails", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "RESULT",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		node.setVariable("RESULT", `{"summary":"hello"}`)
		err := node.validateOutput(context.Background())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "output validation failed")
	})

	t.Run("empty output name skips validation", func(t *testing.T) {
		node := NewNode(core.Step{
			Name:         "test",
			Output:       "",
			OutputSchema: schema,
		}, NodeState{})
		node.Init()
		err := node.validateOutput(context.Background())
		assert.NoError(t, err)
	})
}
