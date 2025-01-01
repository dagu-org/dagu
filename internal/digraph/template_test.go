// Copyright (C) 2025 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplace(t *testing.T) {
	t.Run("simple variable replacement", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
			"AGE":  30,
		}

		result, err := renderTemplate("Hello {{.NAME}}, you are {{.AGE}} years old", data)
		assert.NoError(t, err)
		assert.Equal(t, "Hello John, you are 30 years old", result)
	})

	t.Run("nested map access", func(t *testing.T) {
		data := map[string]any{
			"CONFIG": map[string]any{
				"PATH": "/data",
				"FILE": "output.txt",
			},
		}

		result, err := renderTemplate("File: {{.CONFIG.PATH}}/{{.CONFIG.FILE}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "File: /data/output.txt", result)
	})

	t.Run("with array access", func(t *testing.T) {
		data := map[string]any{
			"ITEMS": []string{"a", "b", "c"},
		}

		// Use index to access array elements
		result, err := renderTemplate("First item: {{index .ITEMS 0}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "First item: a", result)
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		_, err := renderTemplate("Hello {{.NAME} missing closing bracket", data)
		assert.Error(t, err)
	})

	t.Run("undefined variable", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		result, err := renderTemplate("Hello {{.UNDEFINED}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "Hello ", result)
	})

	t.Run("empty template", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		result, err := renderTemplate("", data)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("nil data", func(t *testing.T) {
		result, err := renderTemplate("Hello {{.NAME}}", nil)
		assert.NoError(t, err)
		// The "<no value>" will be replaced with an empty string
		assert.Equal(t, "Hello ", result)
	})
}
