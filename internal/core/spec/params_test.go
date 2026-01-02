package spec

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParamPairString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pair     paramPair
		expected string
	}{
		{
			name:     "NamedParam",
			pair:     paramPair{Name: "foo", Value: "bar"},
			expected: "foo=bar",
		},
		{
			name:     "PositionalParam",
			pair:     paramPair{Name: "", Value: "value"},
			expected: "value",
		},
		{
			name:     "EmptyValue",
			pair:     paramPair{Name: "key", Value: ""},
			expected: "key=",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.pair.String())
		})
	}
}

func TestParamPairEscaped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		pair     paramPair
		expected string
	}{
		{
			name:     "NamedParam",
			pair:     paramPair{Name: "foo", Value: "bar"},
			expected: `foo="bar"`,
		},
		{
			name:     "PositionalParam",
			pair:     paramPair{Name: "", Value: "value"},
			expected: `"value"`,
		},
		{
			name:     "ValueWithSpaces",
			pair:     paramPair{Name: "msg", Value: "hello world"},
			expected: `msg="hello world"`,
		},
		{
			name:     "ValueWithQuotes",
			pair:     paramPair{Name: "json", Value: `{"key":"value"}`},
			expected: `json="{\"key\":\"value\"}"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tt.pair.Escaped())
		})
	}
}

func TestParseStringParams(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		ctx:  context.Background(),
		opts: BuildOpts{Flags: BuildFlagNoEval},
	}

	tests := []struct {
		name     string
		input    string
		expected []paramPair
	}{
		{
			name:     "SinglePositionalParam",
			input:    "value",
			expected: []paramPair{{Name: "", Value: "value"}},
		},
		{
			name:     "SingleNamedParam",
			input:    "key=value",
			expected: []paramPair{{Name: "key", Value: "value"}},
		},
		{
			name:  "MultipleNamedParams",
			input: "foo=bar baz=qux",
			expected: []paramPair{
				{Name: "foo", Value: "bar"},
				{Name: "baz", Value: "qux"},
			},
		},
		{
			name:  "MixedParams",
			input: "positional key=value",
			expected: []paramPair{
				{Name: "", Value: "positional"},
				{Name: "key", Value: "value"},
			},
		},
		{
			name:     "QuotedValue",
			input:    `msg="hello world"`,
			expected: []paramPair{{Name: "msg", Value: "hello world"}},
		},
		{
			name:     "QuotedValueWithEscapedQuotes",
			input:    `msg="say \"hello\""`,
			expected: []paramPair{{Name: "msg", Value: `say "hello\`}},
		},
		{
			name:     "EmptyString",
			input:    "",
			expected: nil,
		},
		{
			name:  "MultiplePositionalParams",
			input: "one two three",
			expected: []paramPair{
				{Name: "", Value: "one"},
				{Name: "", Value: "two"},
				{Name: "", Value: "three"},
			},
		},
		{
			name:     "BacktickValue",
			input:    "cmd=`echo hello`",
			expected: []paramPair{{Name: "cmd", Value: "`echo hello`"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseStringParams(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseStringParamsWithJSON(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		ctx:  context.Background(),
		opts: BuildOpts{Flags: BuildFlagNoEval},
	}

	tests := []struct {
		name     string
		input    string
		expected []paramPair
	}{
		{
			name:  "JSONObject",
			input: `{"key1": "test1", "key2": "test2"}`,
			expected: []paramPair{
				{Name: "key1", Value: "test1"},
				{Name: "key2", Value: "test2"},
			},
		},
		{
			name:  "JSONArray",
			input: `["val1", "val2", "val3"]`,
			expected: []paramPair{
				{Name: "", Value: "val1"},
				{Name: "", Value: "val2"},
				{Name: "", Value: "val3"},
			},
		},
		{
			name:  "JSONWithNonStringValues",
			input: `{"count": 42, "enabled": true}`,
			expected: []paramPair{
				{Name: "count", Value: "42"},
				{Name: "enabled", Value: "true"},
			},
		},
		{
			name:     "InvalidJSONFallsBackToRegex",
			input:    `{invalid json`,
			expected: []paramPair{{Name: "", Value: "{invalid"}, {Name: "", Value: "json"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseStringParams(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseStringParamsWithEval(t *testing.T) {
	t.Run("BacktickCommandSubstitution", func(t *testing.T) {
		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{},
		}

		result, err := parseStringParams(ctx, "val=`echo hello`")
		require.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, "val", result[0].Name)
		assert.Equal(t, "hello", result[0].Value)
	})
}

func TestParseListParams(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		ctx:  context.Background(),
		opts: BuildOpts{Flags: BuildFlagNoEval},
	}

	tests := []struct {
		name     string
		input    []string
		expected []paramPair
	}{
		{
			name:     "EmptyList",
			input:    []string{},
			expected: nil,
		},
		{
			name:     "SingleItem",
			input:    []string{"foo=bar"},
			expected: []paramPair{{Name: "foo", Value: "bar"}},
		},
		{
			name:  "MultipleItems",
			input: []string{"foo=bar", "baz=qux"},
			expected: []paramPair{
				{Name: "foo", Value: "bar"},
				{Name: "baz", Value: "qux"},
			},
		},
		{
			name:  "ItemsWithMultipleParams",
			input: []string{"a=1 b=2", "c=3"},
			expected: []paramPair{
				{Name: "a", Value: "1"},
				{Name: "b", Value: "2"},
				{Name: "c", Value: "3"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := parseListParams(ctx, tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMapParams(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		ctx:  context.Background(),
		opts: BuildOpts{Flags: BuildFlagNoEval},
	}

	t.Run("EmptySlice", func(t *testing.T) {
		t.Parallel()
		result, err := parseMapParams(ctx, []any{})
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("SingleMap", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"foo": "bar"},
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, []paramPair{{Name: "foo", Value: "bar"}}, result)
	})

	t.Run("MapWithMultipleKeys", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"a": "1", "b": "2"},
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		// Keys are sorted alphabetically
		assert.Equal(t, []paramPair{
			{Name: "a", Value: "1"},
			{Name: "b", Value: "2"},
		}, result)
	})

	t.Run("MultipleMaps", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"foo": "bar"},
			map[string]any{"baz": "qux"},
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, result)
	})

	t.Run("MixedMapAndString", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"foo": "bar"},
			"baz=qux",
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, result)
	})

	t.Run("IntegerValue", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"count": 42},
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, []paramPair{{Name: "count", Value: "42"}}, result)
	})

	t.Run("BooleanValue", func(t *testing.T) {
		t.Parallel()
		input := []any{
			map[string]any{"debug": true},
		}
		result, err := parseMapParams(ctx, input)
		require.NoError(t, err)
		assert.Equal(t, []paramPair{{Name: "debug", Value: "true"}}, result)
	})

	t.Run("InvalidType", func(t *testing.T) {
		t.Parallel()
		input := []any{123}
		_, err := parseMapParams(ctx, input)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameter value")
	})
}

func TestParseParamValue(t *testing.T) {
	t.Parallel()

	ctx := BuildContext{
		ctx:  context.Background(),
		opts: BuildOpts{Flags: BuildFlagNoEval},
	}

	t.Run("Nil", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("String", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, "foo=bar baz=qux")
		require.NoError(t, err)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, result)
	})

	t.Run("StringSlice", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, []string{"foo=bar", "baz=qux"})
		require.NoError(t, err)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, result)
	})

	t.Run("AnySlice", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, []any{
			map[string]any{"foo": "bar"},
		})
		require.NoError(t, err)
		assert.Equal(t, []paramPair{{Name: "foo", Value: "bar"}}, result)
	})

	t.Run("MapWithoutSchema", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, map[string]any{
			"foo": "bar",
			"baz": "qux",
		})
		require.NoError(t, err)
		// Keys are sorted
		assert.Len(t, result, 2)
	})

	t.Run("MapWithSchemaNoValues", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, map[string]any{
			"schema": "schema.json",
		})
		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("MapWithSchemaAndValues", func(t *testing.T) {
		t.Parallel()
		result, err := parseParamValue(ctx, map[string]any{
			"schema": "schema.json",
			"values": map[string]any{"foo": "bar"},
		})
		require.NoError(t, err)
		assert.Equal(t, []paramPair{{Name: "foo", Value: "bar"}}, result)
	})

	t.Run("InvalidType", func(t *testing.T) {
		t.Parallel()
		_, err := parseParamValue(ctx, 123)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameter value")
	})
}

func TestOverrideParams(t *testing.T) {
	t.Parallel()

	t.Run("OverrideByName", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{
			{Name: "foo", Value: "original"},
			{Name: "bar", Value: "keep"},
		}
		override := []paramPair{
			{Name: "foo", Value: "overridden"},
		}
		overrideParams(&params, override)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "overridden"},
			{Name: "bar", Value: "keep"},
		}, params)
	})

	t.Run("AddNewNamedParam", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{
			{Name: "foo", Value: "bar"},
		}
		override := []paramPair{
			{Name: "baz", Value: "qux"},
		}
		overrideParams(&params, override)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, params)
	})

	t.Run("OverrideByPosition", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{
			{Name: "", Value: "first"},
			{Name: "", Value: "second"},
		}
		override := []paramPair{
			{Name: "", Value: "new-first"},
		}
		overrideParams(&params, override)
		assert.Equal(t, []paramPair{
			{Name: "", Value: "new-first"},
			{Name: "", Value: "second"},
		}, params)
	})

	t.Run("AddPositionalBeyondLength", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{
			{Name: "", Value: "first"},
		}
		override := []paramPair{
			{Name: "", Value: "new-first"},
			{Name: "", Value: "new-second"},
		}
		overrideParams(&params, override)
		assert.Equal(t, []paramPair{
			{Name: "", Value: "new-first"},
			{Name: "", Value: "new-second"},
		}, params)
	})

	t.Run("EmptyOverride", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{
			{Name: "foo", Value: "bar"},
		}
		overrideParams(&params, []paramPair{})
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
		}, params)
	})

	t.Run("EmptyParams", func(t *testing.T) {
		t.Parallel()
		params := []paramPair{}
		override := []paramPair{
			{Name: "foo", Value: "bar"},
		}
		overrideParams(&params, override)
		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
		}, params)
	})
}

func TestParseParams(t *testing.T) {
	t.Parallel()

	t.Run("SimpleParams", func(t *testing.T) {
		t.Parallel()

		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{Flags: BuildFlagNoEval},
		}

		var params []paramPair
		var envs []string

		err := parseParams(ctx, "foo=bar baz=qux", &params, &envs)
		require.NoError(t, err)

		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, params)
		// NoEval flag prevents env vars from being added
		assert.Empty(t, envs)
	})

	t.Run("PositionalParamsGetNames", func(t *testing.T) {
		t.Parallel()

		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{Flags: BuildFlagNoEval},
		}

		var params []paramPair
		var envs []string

		err := parseParams(ctx, "one two three", &params, &envs)
		require.NoError(t, err)

		// Positional params get numbered names
		assert.Equal(t, []paramPair{
			{Name: "1", Value: "one"},
			{Name: "2", Value: "two"},
			{Name: "3", Value: "three"},
		}, params)
	})

	t.Run("WithEvalAddsEnvs", func(t *testing.T) {
		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{},
		}

		var params []paramPair
		var envs []string

		err := parseParams(ctx, "foo=bar baz=qux", &params, &envs)
		require.NoError(t, err)

		assert.Equal(t, []paramPair{
			{Name: "foo", Value: "bar"},
			{Name: "baz", Value: "qux"},
		}, params)
		assert.Equal(t, []string{"foo=bar", "baz=qux"}, envs)
	})

	t.Run("VariableReference", func(t *testing.T) {
		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{},
		}

		var params []paramPair
		var envs []string

		// Second param references first param
		err := parseParams(ctx, []any{
			map[string]any{"BASE": "/opt"},
			map[string]any{"PATH_VAR": "${BASE}/bin"},
		}, &params, &envs)
		require.NoError(t, err)

		assert.Equal(t, "/opt", params[0].Value)
		assert.Equal(t, "/opt/bin", params[1].Value)
	})

	t.Run("NilInput", func(t *testing.T) {
		t.Parallel()

		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{Flags: BuildFlagNoEval},
		}

		var params []paramPair
		var envs []string

		err := parseParams(ctx, nil, &params, &envs)
		require.NoError(t, err)
		assert.Empty(t, params)
		assert.Empty(t, envs)
	})

	t.Run("InvalidInput", func(t *testing.T) {
		t.Parallel()

		ctx := BuildContext{
			ctx:  context.Background(),
			opts: BuildOpts{Flags: BuildFlagNoEval},
		}

		var params []paramPair
		var envs []string

		err := parseParams(ctx, 123, &params, &envs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid parameter value")
	})
}

func TestEvalParamValue(t *testing.T) {
	t.Run("SimpleValue", func(t *testing.T) {
		ctx := BuildContext{
			ctx: context.Background(),
		}

		result, err := evalParamValue(ctx, "hello", nil)
		require.NoError(t, err)
		assert.Equal(t, "hello", result)
	})

	t.Run("WithAccumulatedVars", func(t *testing.T) {
		ctx := BuildContext{
			ctx: context.Background(),
		}

		vars := map[string]string{"BASE": "/opt"}
		result, err := evalParamValue(ctx, "${BASE}/bin", vars)
		require.NoError(t, err)
		assert.Equal(t, "/opt/bin", result)
	})

	t.Run("WithBuildEnv", func(t *testing.T) {
		ctx := BuildContext{
			ctx:      context.Background(),
			buildEnv: map[string]string{"ENV_VAR": "value"},
		}

		result, err := evalParamValue(ctx, "${ENV_VAR}", nil)
		require.NoError(t, err)
		assert.Equal(t, "value", result)
	})

	t.Run("AccumulatedVarsPrecedence", func(t *testing.T) {
		ctx := BuildContext{
			ctx:      context.Background(),
			buildEnv: map[string]string{"VAR": "from-env"},
		}

		vars := map[string]string{"VAR": "from-accumulated"}
		result, err := evalParamValue(ctx, "${VAR}", vars)
		require.NoError(t, err)
		// Accumulated vars should take precedence (added first to options)
		assert.Equal(t, "from-accumulated", result)
	})
}
