package executor

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/assert"
)

func TestEvalString(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("TEST_VAR", "TEST_VAR=hello")
	env.Envs["ANOTHER_VAR"] = "world"
	ctx = WithEnv(ctx, env)

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
			result, err := EvalString(ctx, tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvalBool(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("TRUE_VAR", "TRUE_VAR=true")
	env.Variables.Store("FALSE_VAR", "FALSE_VAR=false")
	env.Variables.Store("ONE_VAR", "ONE_VAR=1")
	env.Variables.Store("ZERO_VAR", "ZERO_VAR=0")
	env.Variables.Store("INVALID_VAR", "INVALID_VAR=not-a-bool")
	ctx = WithEnv(ctx, env)

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
			result, err := EvalBool(ctx, tt.input)
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
	env := NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("NAME_VAR", "NAME_VAR=John")
	env.Variables.Store("DESC_VAR", "DESC_VAR=Developer")
	env.Variables.Store("NESTED_VAR", "NESTED_VAR=NestedValue")
	ctx = WithEnv(ctx, env)

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
	result, err := EvalObject(ctx, testObj)
	assert.NoError(t, err)
	assert.Equal(t, expected, result)

	// Test with a non-struct type
	_, err = EvalObject(ctx, "not a struct")
	assert.NoError(t, err)
}

// TestEvalObjectWithExecutorConfig tests that EvalObject works correctly with the ExecutorConfig struct
func TestEvalObjectWithExecutorConfig(t *testing.T) {
	// Create a test context with environment variables
	ctx := context.Background()
	env := NewEnv(ctx, digraph.Step{Name: "test-step"})
	env.Variables.Store("EXECUTOR_TYPE", "EXECUTOR_TYPE=docker")
	env.Variables.Store("HOST_VAR", "HOST_VAR=localhost")
	env.Variables.Store("PORT_VAR", "PORT_VAR=8080")
	ctx = WithEnv(ctx, env)

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
	result, err := EvalObject(ctx, config.Config)
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
