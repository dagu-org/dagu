package runtime_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/runtime"
)

// Note: This test uses runtime.NewContext to set up the DAG context,
// then uses runtime.GetEnv/WithEnv to manage the runtime Env.

func TestDebugVariables(t *testing.T) {
	// Create a test context with environment variables
	ctx := runtime.NewContext(context.Background(), &core.DAG{}, "test-run", "test.log")
	env := runtime.GetEnv(ctx)

	// Store variable with spaces using Scope
	env.Scope = env.Scope.WithEntry("SPACES", "  ", cmdutil.EnvSourceStepEnv)

	// IMPORTANT: Update the context with the modified env
	ctx = runtime.WithEnv(ctx, env)

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

	// Let's also check what Scope has
	fmt.Printf("\nScope ToMap: %#v\n", env.Scope.ToMap())

	// Let's check what GetEnv returns
	envFromCtx := runtime.GetEnv(ctx)
	fmt.Printf("\nEnv from context Scope ToMap: %#v\n", envFromCtx.Scope.ToMap())

	// Test with special characters too
	env.Scope = env.Scope.WithEntry("SPECIAL", "$pecial!@#", cmdutil.EnvSourceStepEnv)

	// Update context again after adding new variable
	ctx = runtime.WithEnv(ctx, env)

	fmt.Printf("\nScope ToMap with special: %#v\n", env.Scope.ToMap())

	result2, err := runtime.EvalString(ctx, "${SPECIAL}")
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	fmt.Printf("\nSpecial Input: '${SPECIAL}'\n")
	fmt.Printf("Special Result: '%s' (len=%d)\n", result2, len(result2))

	// Let's debug the $pecial issue
	// Try different patterns
	env.Scope = env.Scope.WithEntries(map[string]string{
		"DOLLAR":   "$",
		"DOLLAR_P": "$p",
		"ESCAPED":  "\\$pecial",
	}, cmdutil.EnvSourceStepEnv)
	ctx = runtime.WithEnv(ctx, env)

	r3, _ := runtime.EvalString(ctx, "${DOLLAR}")
	r4, _ := runtime.EvalString(ctx, "${DOLLAR_P}")
	r5, _ := runtime.EvalString(ctx, "${ESCAPED}")

	fmt.Printf("\n'${DOLLAR}' -> '%s'\n", r3)
	fmt.Printf("'${DOLLAR_P}' -> '%s'\n", r4)
	fmt.Printf("'${ESCAPED}' -> '%s'\n", r5)

	// Test without environment expansion
	fmt.Printf("\n--- Testing without env expansion ---\n")
	env.Scope = env.Scope.WithEntry("TEST_DOLLAR", "$HOME/test", cmdutil.EnvSourceStepEnv)
	ctx = runtime.WithEnv(ctx, env)

	// This should expand $HOME
	r6, _ := runtime.EvalString(ctx, "${TEST_DOLLAR}")
	fmt.Printf("With env expansion: '${TEST_DOLLAR}' -> '%s'\n", r6)
}
