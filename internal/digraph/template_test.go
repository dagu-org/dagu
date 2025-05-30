package digraph_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestReplace(t *testing.T) {
	t.Run("simple variable replacement", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
			"AGE":  30,
		}

		result, err := digraph.RenderTemplate("Hello {{.NAME}}, you are {{.AGE}} years old", data)
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

		result, err := digraph.RenderTemplate("File: {{.CONFIG.PATH}}/{{.CONFIG.FILE}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "File: /data/output.txt", result)
	})

	t.Run("with array access", func(t *testing.T) {
		data := map[string]any{
			"ITEMS": []string{"a", "b", "c"},
		}

		// Use index to access array elements
		result, err := digraph.RenderTemplate("First item: {{index .ITEMS 0}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "First item: a", result)
	})

	t.Run("invalid template syntax", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		_, err := digraph.RenderTemplate("Hello {{.NAME} missing closing bracket", data)
		assert.Error(t, err)
	})

	t.Run("undefined variable", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		result, err := digraph.RenderTemplate("Hello {{.UNDEFINED}}", data)
		assert.NoError(t, err)
		assert.Equal(t, "Hello ", result)
	})

	t.Run("empty template", func(t *testing.T) {
		data := map[string]any{
			"NAME": "John",
		}

		result, err := digraph.RenderTemplate("", data)
		assert.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("nil data", func(t *testing.T) {
		result, err := digraph.RenderTemplate("Hello {{.NAME}}", nil)
		assert.NoError(t, err)
		// The "<no value>" will be replaced with an empty string
		assert.Equal(t, "Hello ", result)
	})
}
