package scheduler_test

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/executor"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/assert"
)

func TestEvalString(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("TEST_VAR", "TEST_VAR=hello")
	env.Envs["ANOTHER_VAR"] = "world"
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple variable",
			input:    "${TEST_VAR}",
			expected: "hello",
		},
		{
			name:     "another variable",
			input:    "${ANOTHER_VAR}",
			expected: "world",
		},
		{
			name:     "combined variables",
			input:    "${TEST_VAR} ${ANOTHER_VAR}",
			expected: "hello world",
		},
		{
			name:     "no variables",
			input:    "no variables here",
			expected: "no variables here",
		},
		{
			name:     "non-existent variable",
			input:    "${NON_EXISTENT}",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalString(ctx, tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalBool(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("TRUE_VAR", "TRUE_VAR=true")
	env.Variables.Store("FALSE_VAR", "FALSE_VAR=false")
	env.Variables.Store("ONE_VAR", "ONE_VAR=1")
	env.Variables.Store("ZERO_VAR", "ZERO_VAR=0")
	env.Variables.Store("INVALID_VAR", "INVALID_VAR=not-a-bool")
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    any
		expected bool
		wantErr  bool
	}{
		{
			name:     "true boolean",
			input:    true,
			expected: true,
			wantErr:  false,
		},
		{
			name:     "false boolean",
			input:    false,
			expected: false,
			wantErr:  false,
		},
		{
			name:     "true string",
			input:    "${TRUE_VAR}",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "false string",
			input:    "${FALSE_VAR}",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "1 string",
			input:    "${ONE_VAR}",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "0 string",
			input:    "${ZERO_VAR}",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "invalid string",
			input:    "${INVALID_VAR}",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "unsupported type",
			input:    123,
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalBool(ctx, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestStruct is a test struct for EvalObject
type TestStruct struct {
	Name        string
	Description string
	Count       int
	Nested      NestedStruct
}

type NestedStruct struct {
	Field string
}

func TestEvalObject(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("NAME_VAR", "NAME_VAR=John")
	env.Variables.Store("DESC_VAR", "DESC_VAR=Developer")
	env.Variables.Store("NESTED_VAR", "NESTED_VAR=NestedValue")
	ctx = executor.WithEnv(ctx, env)

	// Create a test struct
	testObj := TestStruct{
		Name:        "${NAME_VAR}",
		Description: "A ${DESC_VAR}",
		Count:       42, // This should remain unchanged
		Nested: NestedStruct{
			Field: "${NESTED_VAR}",
		},
	}

	// Expected result
	expected := TestStruct{
		Name:        "John",
		Description: "A Developer",
		Count:       42,
		Nested: NestedStruct{
			Field: "NestedValue",
		},
	}

	// Test EvalObject
	result, err := scheduler.EvalObject(ctx, testObj)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	// Test with a non-struct type
	_, err = scheduler.EvalObject(ctx, "not a struct")
	assert.NoError(t, err)
}

// TestEvalObjectWithExecutorConfig tests that EvalObject works correctly with the ExecutorConfig struct
func TestEvalObjectWithExecutorConfig(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("EXECUTOR_TYPE", "EXECUTOR_TYPE=docker")
	env.Variables.Store("HOST_VAR", "HOST_VAR=localhost")
	env.Variables.Store("PORT_VAR", "PORT_VAR=8080")
	ctx = executor.WithEnv(ctx, env)

	// Create an ExecutorConfig with variables
	config := digraph.ExecutorConfig{
		Type: "${EXECUTOR_TYPE}",
		Config: map[string]any{
			"host": "${HOST_VAR}",
			"port": "${PORT_VAR}",
			"nested": map[string]any{
				"value": "${HOST_VAR}:${PORT_VAR}",
			},
		},
	}

	// Expected result
	expected := digraph.ExecutorConfig{
		Type: "docker",
		Config: map[string]any{
			"host": "localhost",
			"port": "8080",
			"nested": map[string]any{
				"value": "localhost:8080",
			},
		},
	}

	// Test EvalObject
	result, err := scheduler.EvalObject(ctx, config.Config)
	assert.NoError(t, err)

	// Check Config map values
	assert.Equal(t, expected.Config["host"], result["host"])
	assert.Equal(t, expected.Config["port"], result["port"])

	// Check nested map
	nestedResult, ok := result["nested"].(map[string]any)
	assert.True(t, ok, "nested should be a map[string]any")

	nestedExpected, ok := expected.Config["nested"].(map[string]any)
	assert.True(t, ok, "expected nested should be a map[string]any")

	assert.Equal(t, nestedExpected["value"], nestedResult["value"])
}

func TestGenerateChildDAGRunID(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.DAGRunID = "parent-run-123"
	env.Step.Name = "child-step"
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name      string
		params    string
		repeated  bool
		expectLen int // Expected length of the hash
	}{
		{
			name:      "non-repeated run",
			params:    "param1=value1",
			repeated:  false,
			expectLen: 11, // Base58 encoded SHA256 should be consistent length
		},
		{
			name:      "repeated run",
			params:    "param1=value1",
			repeated:  true,
			expectLen: 11, // Base58 encoded SHA256 should be consistent length
		},
		{
			name:      "empty params",
			params:    "",
			repeated:  false,
			expectLen: 11,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := scheduler.GenerateChildDAGRunID(ctx, tt.params, tt.repeated)
			assert.NotEmpty(t, result)
			// Base58 encoded strings should be at least this length
			assert.GreaterOrEqual(t, len(result), tt.expectLen)

			// For non-repeated runs, the same parameters should generate the same ID
			if !tt.repeated {
				result2 := scheduler.GenerateChildDAGRunID(ctx, tt.params, tt.repeated)
				assert.Equal(t, result, result2)
			} else {
				// For repeated runs, the same parameters should generate different IDs
				result2 := scheduler.GenerateChildDAGRunID(ctx, tt.params, tt.repeated)
				assert.NotEqual(t, result, result2)
			}
		})
	}
}

func TestEvalObjectWithComplexNestedStructures(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("VAR1", "VAR1=value1")
	env.Variables.Store("VAR2", "VAR2=value2")
	env.Variables.Store("NUM", "NUM=42")
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name: "deeply nested maps",
			input: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": "${VAR1}",
					},
				},
			},
			expected: map[string]any{
				"level1": map[string]any{
					"level2": map[string]any{
						"level3": "value1",
					},
				},
			},
		},
		{
			name: "array of maps",
			input: map[string]any{
				"items": []any{
					map[string]any{"name": "${VAR1}"},
					map[string]any{"name": "${VAR2}"},
				},
			},
			expected: map[string]any{
				"items": []any{
					map[string]any{"name": "value1"},
					map[string]any{"name": "value2"},
				},
			},
		},
		{
			name: "slice of strings with variables",
			input: map[string]any{
				"commands": []string{
					"echo ${VAR1}",
					"echo ${VAR2}",
					"echo ${NUM}",
				},
			},
			expected: map[string]any{
				"commands": []string{
					"echo value1",
					"echo value2",
					"echo 42",
				},
			},
		},
		{
			name: "nested slices",
			input: map[string]any{
				"matrix": [][]string{
					{"${VAR1}", "${VAR2}"},
					{"${NUM}", "static"},
				},
			},
			expected: map[string]any{
				"matrix": [][]string{
					{"value1", "value2"},
					{"42", "static"},
				},
			},
		},
		{
			name: "slice of interfaces",
			input: map[string]any{
				"mixed": []any{
					"${VAR1}",
					42,
					true,
					map[string]any{"key": "${VAR2}"},
				},
			},
			expected: map[string]any{
				"mixed": []any{
					"value1",
					42,
					true,
					map[string]any{"key": "value2"},
				},
			},
		},
		{
			name: "empty slice",
			input: map[string]any{
				"empty": []string{},
			},
			expected: map[string]any{
				"empty": []string{},
			},
		},
		{
			name: "mixed types in map",
			input: map[string]any{
				"string": "${VAR1}",
				"number": 123,
				"bool":   true,
				"null":   nil,
				"array":  []string{"a", "b"},
			},
			expected: map[string]any{
				"string": "value1",
				"number": 123,
				"bool":   true,
				"null":   nil,
				"array":  []string{"a", "b"},
			},
		},
		{
			name: "map with nil values",
			input: map[string]any{
				"key1": nil,
				"key2": "${VAR1}",
			},
			expected: map[string]any{
				"key1": nil,
				"key2": "value1",
			},
		},
		{
			name: "map with pointer values",
			input: map[string]any{
				"ptr": &TestStruct{
					Name: "${VAR1}",
				},
			},
			expected: map[string]any{
				"ptr": TestStruct{
					Name: "value1",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalObject(ctx, tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalStringEdgeCases(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := digraph.SetupEnvForTest(context.Background(), &digraph.DAG{}, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)
	env := executor.GetEnv(ctx)
	env.Variables.Store("EMPTY", "EMPTY=")
	env.Variables.Store("SPACES", "SPACES=  ")
	env.Variables.Store("SPECIAL", "SPECIAL=special!@#")
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "empty variable",
			input:    "${EMPTY}",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "variable with spaces",
			input:    "${SPACES}",
			expected: "  ",
			wantErr:  false,
		},
		{
			name:     "variable with special characters",
			input:    "${SPECIAL}",
			expected: "special!@#",
			wantErr:  false,
		},
		{
			name:     "nested variable references",
			input:    "${${EMPTY}}",
			expected: "}",
			wantErr:  false,
		},
		{
			name:     "malformed variable",
			input:    "${",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalString(ctx, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEvalObjectWithDirectStringEvaluation(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := executor.NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("STRING_VAR", "STRING_VAR=evaluated_string")
	env.Variables.Store("PATH_VAR", "PATH_VAR=/path/to/file")
	env.Variables.Store("COMBINED", "COMBINED=prefix")
	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    any
		expected any
		wantErr  bool
	}{
		{
			name:     "direct string evaluation",
			input:    "${STRING_VAR}",
			expected: "evaluated_string",
			wantErr:  false,
		},
		{
			name:     "string with multiple variables",
			input:    "${PATH_VAR}/config-${STRING_VAR}.json",
			expected: "/path/to/file/config-evaluated_string.json",
			wantErr:  false,
		},
		{
			name:     "plain string without variables",
			input:    "no variables here",
			expected: "no variables here",
			wantErr:  false,
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string within map",
			input:    map[string]any{"key": "${STRING_VAR}"},
			expected: map[string]any{"key": "evaluated_string"},
			wantErr:  false,
		},
		{
			name: "string within nested structure",
			input: map[string]any{
				"config": map[string]any{
					"path": "${PATH_VAR}",
					"items": []string{
						"${STRING_VAR}",
						"${COMBINED}_suffix",
					},
				},
			},
			expected: map[string]any{
				"config": map[string]any{
					"path": "/path/to/file",
					"items": []string{
						"evaluated_string",
						"prefix_suffix",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalObject(ctx, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestEvalBoolEdgeCases(t *testing.T) {
	t.Parallel()

	// Create a test context with environment variables
	ctx := digraph.SetupEnvForTest(context.Background(), &digraph.DAG{}, nil, digraph.DAGRunRef{}, "test-run", "test.log", nil)

	env := executor.GetEnv(ctx)
	env.Variables.Store("YES", "YES=yes")
	env.Variables.Store("NO", "NO=no")
	env.Variables.Store("ON", "ON=on")
	env.Variables.Store("OFF", "OFF=off")
	env.Variables.Store("T", "T=t")
	env.Variables.Store("F", "F=f")

	ctx = executor.WithEnv(ctx, env)

	tests := []struct {
		name     string
		input    any
		expected bool
		wantErr  bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: false,
			wantErr:  true,
		},
		{
			name:     "string yes",
			input:    "${YES}",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "string no",
			input:    "${NO}",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "string on",
			input:    "${ON}",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "string off",
			input:    "${OFF}",
			expected: false,
			wantErr:  true,
		},
		{
			name:     "string t",
			input:    "${T}",
			expected: true,
			wantErr:  false,
		},
		{
			name:     "string f",
			input:    "${F}",
			expected: false,
			wantErr:  false,
		},
		{
			name:     "struct type",
			input:    TestStruct{},
			expected: false,
			wantErr:  true,
		},
		{
			name:     "slice type",
			input:    []string{"true"},
			expected: false,
			wantErr:  true,
		},
		{
			name:     "map type",
			input:    map[string]string{"key": "true"},
			expected: false,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := scheduler.EvalBool(ctx, tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}
