// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseOutputConfig_WithSchema(t *testing.T) {
	t.Run("object form with schema", func(t *testing.T) {
		input := map[string]any{
			"name": "RESULT",
			"schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"summary": map[string]any{"type": "string"},
				},
				"required": []any{"summary"},
			},
		}
		cfg, err := parseOutputConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "RESULT", cfg.Name)
		assert.NotNil(t, cfg.Schema)
	})

	t.Run("object form without schema", func(t *testing.T) {
		input := map[string]any{"name": "RESULT"}
		cfg, err := parseOutputConfig(input)
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "RESULT", cfg.Name)
		assert.Nil(t, cfg.Schema)
	})

	t.Run("string form has no schema", func(t *testing.T) {
		cfg, err := parseOutputConfig("RESULT")
		require.NoError(t, err)
		require.NotNil(t, cfg)
		assert.Equal(t, "RESULT", cfg.Name)
		assert.Nil(t, cfg.Schema)
	})

	t.Run("nil output", func(t *testing.T) {
		cfg, err := parseOutputConfig(nil)
		require.NoError(t, err)
		assert.Nil(t, cfg)
	})
}

func TestBuildStepOutputSchema(t *testing.T) {
	t.Run("compiles valid schema", func(t *testing.T) {
		s := &step{
			Output: map[string]any{
				"name": "RESULT",
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"summary":    map[string]any{"type": "string"},
						"confidence": map[string]any{"type": "number", "minimum": 0.0, "maximum": 1.0},
					},
					"required": []any{"summary", "confidence"},
				},
			},
		}
		ctx := StepBuildContext{}
		resolved, err := buildStepOutputSchema(ctx, s)
		require.NoError(t, err)
		require.NotNil(t, resolved)

		// Valid data passes
		assert.NoError(t, resolved.Validate(map[string]any{"summary": "test", "confidence": 0.95}))
		// Invalid data fails
		assert.Error(t, resolved.Validate(map[string]any{"summary": "test", "confidence": 2.0}))
	})

	t.Run("returns nil for no schema", func(t *testing.T) {
		s := &step{Output: map[string]any{"name": "RESULT"}}
		resolved, err := buildStepOutputSchema(StepBuildContext{}, s)
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("returns nil for string output", func(t *testing.T) {
		s := &step{Output: "RESULT"}
		resolved, err := buildStepOutputSchema(StepBuildContext{}, s)
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("returns nil for nil output", func(t *testing.T) {
		s := &step{Output: nil}
		resolved, err := buildStepOutputSchema(StepBuildContext{}, s)
		require.NoError(t, err)
		assert.Nil(t, resolved)
	})

	t.Run("nested object schema resolves", func(t *testing.T) {
		s := &step{
			Output: map[string]any{
				"name": "RESULT",
				"schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"metadata": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"version": map[string]any{"type": "integer"},
							},
						},
					},
				},
			},
		}
		resolved, err := buildStepOutputSchema(StepBuildContext{}, s)
		require.NoError(t, err)
		require.NotNil(t, resolved)
	})

	t.Run("schema reference resolves from DAG working dir", func(t *testing.T) {
		dir := t.TempDir()
		schemaPath := filepath.Join(dir, "output-schema.json")
		require.NoError(t, os.WriteFile(schemaPath, []byte(`{
			"type": "object",
			"properties": {
				"summary": {"type": "string"}
			},
			"required": ["summary"]
		}`), 0o600))

		s := &step{
			Output: map[string]any{
				"name":   "RESULT",
				"schema": "output-schema.json",
			},
		}
		ctx := StepBuildContext{
			BuildContext: BuildContext{file: filepath.Join(dir, "dag.yaml")},
			dag:          &core.DAG{WorkingDir: dir},
		}

		resolved, err := buildStepOutputSchema(ctx, s)
		require.NoError(t, err)
		require.NotNil(t, resolved)
		assert.NoError(t, resolved.Validate(map[string]any{"summary": "test"}))
	})

	t.Run("invalid schema reference returns error", func(t *testing.T) {
		s := &step{
			Output: map[string]any{
				"name":   "RESULT",
				"schema": "missing-schema.json",
			},
		}
		_, err := buildStepOutputSchema(StepBuildContext{}, s)
		assert.Error(t, err)
	})
}
