package eval

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObject_String(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{
		"NAME":  "World",
		"VALUE": "42",
	}

	t.Run("SimpleVariableSubstitution", func(t *testing.T) {
		input := "Hello, $NAME!"
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("MultipleVariables", func(t *testing.T) {
		input := "$NAME has value $VALUE"
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "World has value 42", result)
	})

	t.Run("BracedVariable", func(t *testing.T) {
		input := "${NAME}Test"
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "WorldTest", result)
	})

	t.Run("NoVariables", func(t *testing.T) {
		input := "plain text"
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "plain text", result)
	})

	t.Run("EmptyString", func(t *testing.T) {
		input := ""
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})
}

func TestObject_Struct(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{
		"HOST": "localhost",
		"PORT": "8080",
	}

	type Config struct {
		Host    string
		Port    string
		Timeout int
	}

	t.Run("StructWithStringFields", func(t *testing.T) {
		input := Config{
			Host:    "$HOST",
			Port:    "$PORT",
			Timeout: 30,
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "localhost", result.Host)
		assert.Equal(t, "8080", result.Port)
		assert.Equal(t, 30, result.Timeout)
	})

	t.Run("StructWithNoVariables", func(t *testing.T) {
		input := Config{
			Host:    "example.com",
			Port:    "443",
			Timeout: 60,
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "example.com", result.Host)
		assert.Equal(t, "443", result.Port)
		assert.Equal(t, 60, result.Timeout)
	})
}

func TestObject_Map(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{
		"KEY":   "resolved_key",
		"VALUE": "resolved_value",
	}

	t.Run("SimpleMapWithStringValues", func(t *testing.T) {
		input := map[string]string{
			"key1": "$KEY",
			"key2": "$VALUE",
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "resolved_key", result["key1"])
		assert.Equal(t, "resolved_value", result["key2"])
	})

	t.Run("MapWithInterfaceValues", func(t *testing.T) {
		input := map[string]any{
			"str":    "$KEY",
			"number": 42,
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "resolved_key", result["str"])
		assert.Equal(t, 42, result["number"])
	})

	t.Run("NestedMap", func(t *testing.T) {
		input := map[string]any{
			"outer": map[string]any{
				"inner": "$VALUE",
			},
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)

		outerMap, ok := result["outer"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "resolved_value", outerMap["inner"])
	})

	t.Run("EmptyMap", func(t *testing.T) {
		input := map[string]string{}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestObject_Slice(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{
		"ITEM1": "first",
		"ITEM2": "second",
	}

	t.Run("SliceOfStrings", func(t *testing.T) {
		input := []string{"$ITEM1", "$ITEM2", "literal"}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, []string{"first", "second", "literal"}, result)
	})

	t.Run("SliceOfInterface", func(t *testing.T) {
		input := []any{"$ITEM1", 42, "$ITEM2"}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, "first", result[0])
		assert.Equal(t, 42, result[1])
		assert.Equal(t, "second", result[2])
	})

	t.Run("EmptySlice", func(t *testing.T) {
		input := []string{}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("SliceInMap", func(t *testing.T) {
		input := map[string]any{
			"items": []string{"$ITEM1", "$ITEM2"},
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)

		items, ok := result["items"].([]string)
		require.True(t, ok)
		assert.Equal(t, []string{"first", "second"}, items)
	})
}

func TestObject_Primitives(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{"VAR": "value"}

	t.Run("IntegerPassthrough", func(t *testing.T) {
		input := 42
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, 42, result)
	})

	t.Run("FloatPassthrough", func(t *testing.T) {
		input := 3.14
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Equal(t, 3.14, result)
	})

	t.Run("BoolPassthrough", func(t *testing.T) {
		input := true
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.True(t, result)
	})

	t.Run("NilPassthrough", func(t *testing.T) {
		var input *string = nil
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		assert.Nil(t, result)
	})
}

func TestObject_ComplexScenarios(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{
		"HOST":     "api.example.com",
		"PORT":     "443",
		"PROTOCOL": "https",
	}

	type Endpoint struct {
		URL  string
		Name string
	}

	t.Run("StructInMap", func(t *testing.T) {
		input := map[string]any{
			"endpoint": Endpoint{
				URL:  "$PROTOCOL://$HOST:$PORT",
				Name: "main",
			},
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)

		endpoint, ok := result["endpoint"].(Endpoint)
		require.True(t, ok)
		assert.Equal(t, "https://api.example.com:443", endpoint.URL)
		assert.Equal(t, "main", endpoint.Name)
	})

	t.Run("DeeplyNestedStructure", func(t *testing.T) {
		input := map[string]any{
			"level1": map[string]any{
				"level2": map[string]any{
					"value": "$HOST",
				},
			},
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)

		level1, ok := result["level1"].(map[string]any)
		require.True(t, ok)
		level2, ok := level1["level2"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "api.example.com", level2["value"])
	})

	t.Run("SliceOfStructs", func(t *testing.T) {
		input := []Endpoint{
			{URL: "$PROTOCOL://$HOST", Name: "endpoint1"},
			{URL: "$PROTOCOL://$HOST:$PORT", Name: "endpoint2"},
		}
		result, err := Object(ctx, input, vars)
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, "https://api.example.com", result[0].URL)
		assert.Equal(t, "https://api.example.com:443", result[1].URL)
	})
}

func TestObject_EmptyVars(t *testing.T) {
	ctx := context.Background()
	emptyVars := map[string]string{}

	t.Run("StringWithUndefinedVariable", func(t *testing.T) {
		input := "Hello, $UNDEFINED!"
		result, err := Object(ctx, input, emptyVars)
		require.NoError(t, err)
		// Undefined variables are preserved as-is (consistent with struct field behavior)
		assert.Equal(t, "Hello, $UNDEFINED!", result)
	})
}

func TestObject_NilVars(t *testing.T) {
	ctx := context.Background()

	t.Run("NilVarsMap", func(t *testing.T) {
		input := "Hello, World!"
		result, err := Object(ctx, input, nil)
		require.NoError(t, err)
		assert.Equal(t, "Hello, World!", result)
	})
}
