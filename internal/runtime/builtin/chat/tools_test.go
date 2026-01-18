package chat

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseToolParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []toolParam
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: nil,
		},
		{
			name:  "SingleRequiredParam",
			input: "query",
			expected: []toolParam{
				{Name: "query", Type: "string", Required: true},
			},
		},
		{
			name:  "SingleParamWithDefault",
			input: "max_results=10",
			expected: []toolParam{
				{Name: "max_results", Type: "integer", Default: int64(10), Required: false},
			},
		},
		{
			name:  "MultipleParams",
			input: "query max_results=10 include_images=true",
			expected: []toolParam{
				{Name: "query", Type: "string", Required: true},
				{Name: "max_results", Type: "integer", Default: int64(10), Required: false},
				{Name: "include_images", Type: "boolean", Default: true, Required: false},
			},
		},
		{
			name:  "StringWithQuotes",
			input: `name="default value"`,
			expected: []toolParam{
				{Name: "name", Type: "string", Default: "default value", Required: false},
			},
		},
		{
			name:  "FloatDefault",
			input: "temperature=0.7",
			expected: []toolParam{
				{Name: "temperature", Type: "number", Default: 0.7, Required: false},
			},
		},
		{
			name:  "BooleanFalse",
			input: "verbose=false",
			expected: []toolParam{
				{Name: "verbose", Type: "boolean", Default: false, Required: false},
			},
		},
		{
			name:  "EmptyArrayDefault",
			input: "filters=[]",
			expected: []toolParam{
				{Name: "filters", Type: "array", Default: []any{}, Required: false},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result, err := parseToolParams(tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestInferTypeFromDefault(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		value        string
		expectedVal  any
		expectedType string
	}{
		{
			name:         "Integer",
			value:        "42",
			expectedVal:  int64(42),
			expectedType: "integer",
		},
		{
			name:         "NegativeInteger",
			value:        "-5",
			expectedVal:  int64(-5),
			expectedType: "integer",
		},
		{
			name:         "Float",
			value:        "3.14",
			expectedVal:  3.14,
			expectedType: "number",
		},
		{
			name:         "BooleanTrue",
			value:        "true",
			expectedVal:  true,
			expectedType: "boolean",
		},
		{
			name:         "BooleanFalse",
			value:        "false",
			expectedVal:  false,
			expectedType: "boolean",
		},
		{
			name:         "DoubleQuotedString",
			value:        `"hello world"`,
			expectedVal:  "hello world",
			expectedType: "string",
		},
		{
			name:         "SingleQuotedString",
			value:        `'test'`,
			expectedVal:  "test",
			expectedType: "string",
		},
		{
			name:         "PlainString",
			value:        "hello",
			expectedVal:  "hello",
			expectedType: "string",
		},
		{
			name:         "EmptyArray",
			value:        "[]",
			expectedVal:  []any{},
			expectedType: "array",
		},
		{
			name:         "ArrayWithValues",
			value:        `["a","b"]`,
			expectedVal:  []any{"a", "b"},
			expectedType: "array",
		},
		{
			name:         "EmptyObject",
			value:        "{}",
			expectedVal:  map[string]any{},
			expectedType: "object",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			val, typ := inferTypeFromDefault(tc.value)
			assert.Equal(t, tc.expectedVal, val)
			assert.Equal(t, tc.expectedType, typ)
		})
	}
}

func TestSplitParams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "EmptyString",
			input:    "",
			expected: nil,
		},
		{
			name:     "SingleParam",
			input:    "query",
			expected: []string{"query"},
		},
		{
			name:     "MultipleParams",
			input:    "a b c",
			expected: []string{"a", "b", "c"},
		},
		{
			name:     "ParamWithQuotedValue",
			input:    `name="hello world" count=5`,
			expected: []string{`name="hello world"`, "count=5"},
		},
		{
			name:     "SingleQuotedValue",
			input:    `greeting='Hello, World!'`,
			expected: []string{`greeting='Hello, World!'`},
		},
		{
			name:     "MultipleSpaces",
			input:    "a   b    c",
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := splitParams(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildJSONSchema(t *testing.T) {
	t.Parallel()

	t.Run("EmptyParams", func(t *testing.T) {
		t.Parallel()

		result := buildJSONSchema(nil)
		assert.Equal(t, "object", result["type"])
		assert.Empty(t, result["properties"])
		assert.Nil(t, result["required"])
	})

	t.Run("RequiredParam", func(t *testing.T) {
		t.Parallel()

		params := []toolParam{
			{Name: "query", Type: "string", Required: true},
		}
		result := buildJSONSchema(params)

		props := result["properties"].(map[string]any)
		assert.Contains(t, props, "query")

		queryProp := props["query"].(map[string]any)
		assert.Equal(t, "string", queryProp["type"])
		assert.Contains(t, queryProp["description"], "query parameter")

		required := result["required"].([]string)
		assert.Contains(t, required, "query")
	})

	t.Run("OptionalParamWithDefault", func(t *testing.T) {
		t.Parallel()

		params := []toolParam{
			{Name: "limit", Type: "integer", Default: int64(10), Required: false},
		}
		result := buildJSONSchema(params)

		props := result["properties"].(map[string]any)
		limitProp := props["limit"].(map[string]any)
		assert.Equal(t, "integer", limitProp["type"])
		assert.Equal(t, int64(10), limitProp["default"])

		assert.Nil(t, result["required"])
	})

	t.Run("MixedParams", func(t *testing.T) {
		t.Parallel()

		params := []toolParam{
			{Name: "query", Type: "string", Required: true},
			{Name: "limit", Type: "integer", Default: int64(10), Required: false},
			{Name: "format", Type: "string", Required: true},
		}
		result := buildJSONSchema(params)

		props := result["properties"].(map[string]any)
		assert.Len(t, props, 3)

		required := result["required"].([]string)
		assert.Len(t, required, 2)
		assert.Contains(t, required, "query")
		assert.Contains(t, required, "format")
	})
}

func TestToolRegistry_ToLLMTools(t *testing.T) {
	t.Parallel()

	t.Run("NilRegistry", func(t *testing.T) {
		t.Parallel()

		var r *ToolRegistry
		result := r.ToLLMTools()
		assert.Nil(t, result)
	})

	t.Run("EmptyRegistry", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			tools: make(map[string]*toolInfo),
		}
		result := r.ToLLMTools()
		assert.Empty(t, result)
	})

	t.Run("WithTools", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			tools: map[string]*toolInfo{
				"search": {
					Name:        "search",
					Description: "Search the web",
					Params: []toolParam{
						{Name: "query", Type: "string", Required: true},
					},
				},
			},
		}
		result := r.ToLLMTools()

		require.Len(t, result, 1)
		assert.Equal(t, "function", result[0].Type)
		assert.Equal(t, "search", result[0].Function.Name)
		assert.Equal(t, "Search the web", result[0].Function.Description)
		assert.Contains(t, result[0].Function.Parameters, "type")
		assert.Contains(t, result[0].Function.Parameters, "properties")
	})
}

func TestToolRegistry_GetDAGByToolName(t *testing.T) {
	t.Parallel()

	t.Run("NilRegistry", func(t *testing.T) {
		t.Parallel()

		var r *ToolRegistry
		dag, ok := r.GetDAGByToolName("test")
		assert.Nil(t, dag)
		assert.False(t, ok)
	})

	t.Run("NotFound", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			tools: make(map[string]*toolInfo),
		}
		dag, ok := r.GetDAGByToolName("unknown")
		assert.Nil(t, dag)
		assert.False(t, ok)
	})

	t.Run("Found", func(t *testing.T) {
		t.Parallel()

		testDAG := &core.DAG{Name: "test-dag"}
		r := &ToolRegistry{
			tools: map[string]*toolInfo{
				"test": {
					Name: "test",
					DAG:  testDAG,
				},
			},
		}
		dag, ok := r.GetDAGByToolName("test")
		assert.Equal(t, testDAG, dag)
		assert.True(t, ok)
	})
}

func TestToolRegistry_GetDAGName(t *testing.T) {
	t.Parallel()

	t.Run("NilRegistry", func(t *testing.T) {
		t.Parallel()

		var r *ToolRegistry
		name, ok := r.GetDAGName("test")
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("Found", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			dagNames: map[string]string{
				"search_tool": "search-tool-dag",
			},
		}
		name, ok := r.GetDAGName("search_tool")
		assert.Equal(t, "search-tool-dag", name)
		assert.True(t, ok)
	})
}

func TestToolRegistry_HasTools(t *testing.T) {
	t.Parallel()

	t.Run("NilRegistry", func(t *testing.T) {
		t.Parallel()

		var r *ToolRegistry
		assert.False(t, r.HasTools())
	})

	t.Run("EmptyRegistry", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			tools: make(map[string]*toolInfo),
		}
		assert.False(t, r.HasTools())
	})

	t.Run("WithTools", func(t *testing.T) {
		t.Parallel()

		r := &ToolRegistry{
			tools: map[string]*toolInfo{
				"test": {},
			},
		}
		assert.True(t, r.HasTools())
	})
}

func TestNewToolRegistry_LocalDAGs(t *testing.T) {
	t.Parallel()

	t.Run("LoadsFromLocalDAGs", func(t *testing.T) {
		t.Parallel()

		// Create a parent DAG with LocalDAGs
		localDAG := &core.DAG{
			Name:          "search_tool",
			Description:   "Search the web",
			DefaultParams: "query",
		}

		parentDAG := &core.DAG{
			Name: "parent",
			LocalDAGs: map[string]*core.DAG{
				"search_tool": localDAG,
			},
		}

		// Create context with parent DAG
		ctx := exec.NewContext(context.Background(), parentDAG, "run-1", "/tmp/log")

		registry, err := NewToolRegistry(ctx, []string{"search_tool"})
		require.NoError(t, err)
		require.NotNil(t, registry)

		// Verify the tool was loaded from LocalDAGs
		assert.True(t, registry.HasTools())
		dag, ok := registry.GetDAGByToolName("search_tool")
		assert.True(t, ok)
		assert.Equal(t, "search_tool", dag.Name)
		assert.Equal(t, "Search the web", dag.Description)
	})

	t.Run("EmptyDagNames", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		registry, err := NewToolRegistry(ctx, []string{})
		assert.NoError(t, err)
		assert.Nil(t, registry)
	})

	t.Run("NilDagNames", func(t *testing.T) {
		t.Parallel()

		ctx := context.Background()
		registry, err := NewToolRegistry(ctx, nil)
		assert.NoError(t, err)
		assert.Nil(t, registry)
	})
}
