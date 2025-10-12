package runtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/runtime"
)

func TestDebugVariables(t *testing.T) {
	// Create a test context with environment variables
	ctx := execution.SetupDAGContext(context.Background(), &core.DAG{}, nil, execution.DAGRunRef{}, "test-run", "test.log", nil, nil)
	env := execution.GetEnv(ctx)

	// Store variable with spaces
	env.Variables.Store("SPACES", "SPACES=  ")

	// IMPORTANT: Update the context with the modified env
	ctx = execution.WithEnv(ctx, env)

	// Print out what Variables() returns
	vars := env.Variables.Variables()
	fmt.Printf("Variables map: %#v\n", vars)

	// Try evaluating directly
	result, err := runtime.EvalString(ctx, "${SPACES}")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	fmt.Printf("Input: '${SPACES}'\n")
	fmt.Printf("Result: '%s' (len=%d)\n", result, len(result))
	fmt.Printf("Result bytes: %v\n", []byte(result))

	// Expected result
	fmt.Printf("Expected: '  ' (len=2)\n")
	fmt.Printf("Expected bytes: %v\n", []byte("  "))

	// Let's also check what Envs map has
	fmt.Printf("\nEnvs map: %#v\n", env.Envs)

	// Let's check what GetEnv returns
	envFromCtx := execution.GetEnv(ctx)
	fmt.Printf("\nEnv from context Variables: %#v\n", envFromCtx.Variables.Variables())

	// Test with special characters too
	env.Variables.Store("SPECIAL", "SPECIAL=$pecial!@#")

	// Update context again after adding new variable
	ctx = execution.WithEnv(ctx, env)

	vars2 := env.Variables.Variables()
	fmt.Printf("\nVariables map with special: %#v\n", vars2)

	result2, err := runtime.EvalString(ctx, "${SPECIAL}")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	fmt.Printf("\nSpecial Input: '${SPECIAL}'\n")
	fmt.Printf("Special Result: '%s' (len=%d)\n", result2, len(result2))

	// Let's debug the $pecial issue
	// Try different patterns
	env.Variables.Store("DOLLAR", "DOLLAR=$")
	env.Variables.Store("DOLLAR_P", "DOLLAR_P=$p")
	env.Variables.Store("ESCAPED", "ESCAPED=\\$pecial")
	ctx = execution.WithEnv(ctx, env)

	r3, _ := runtime.EvalString(ctx, "${DOLLAR}")
	r4, _ := runtime.EvalString(ctx, "${DOLLAR_P}")
	r5, _ := runtime.EvalString(ctx, "${ESCAPED}")

	fmt.Printf("\n'${DOLLAR}' -> '%s'\n", r3)
	fmt.Printf("'${DOLLAR_P}' -> '%s'\n", r4)
	fmt.Printf("'${ESCAPED}' -> '%s'\n", r5)

	// Test without environment expansion
	fmt.Printf("\n--- Testing without env expansion ---\n")
	env.Variables.Store("TEST_DOLLAR", "TEST_DOLLAR=$HOME/test")
	ctx = execution.WithEnv(ctx, env)

	// This should expand $HOME
	r6, _ := runtime.EvalString(ctx, "${TEST_DOLLAR}")
	fmt.Printf("With env expansion: '${TEST_DOLLAR}' -> '%s'\n", r6)
}
