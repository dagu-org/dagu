package cmdutil

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExpandEnvContext(t *testing.T) {
	t.Run("WithScopeInContext", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("MY_VAR", "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "Value is $MY_VAR")
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("ScopeTakesPrecedenceOverOSEnv", func(t *testing.T) {
		key := "TEST_EXPAND_PRECEDENCE"
		os.Setenv(key, "os_value")
		defer os.Unsetenv(key)

		scope := NewEnvScope(nil, false)
		scope.Set(key, "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "Value is $"+key)
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("WithoutScopeFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_OS_FALLBACK"
		os.Setenv(key, "os_value")
		defer os.Unsetenv(key)

		ctx := context.Background()
		result := ExpandEnvContext(ctx, "Value is $"+key)
		assert.Equal(t, "Value is os_value", result)
	})

	t.Run("NilContextFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_NIL_CTX"
		os.Setenv(key, "nil_ctx_value")
		defer os.Unsetenv(key)

		result := ExpandEnvContext(nil, "Value is $"+key)
		assert.Equal(t, "Value is nil_ctx_value", result)
	})

	t.Run("VariableNotFoundReturnsEmpty", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		ctx := WithEnvScope(context.Background(), scope)

		result := ExpandEnvContext(ctx, "Hello $NONEXISTENT!")
		assert.Equal(t, "Hello !", result)
	})

	t.Run("BracedVariableSyntax", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("VAR", "value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "${VAR}Suffix")
		assert.Equal(t, "valueSuffix", result)
	})

	t.Run("MultipleVariables", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("FIRST", "Hello", EnvSourceDAGEnv)
		scope.Set("SECOND", "World", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "$FIRST, $SECOND!")
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("NoVariablesToExpand", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		ctx := WithEnvScope(context.Background(), scope)

		result := ExpandEnvContext(ctx, "plain text without variables")
		assert.Equal(t, "plain text without variables", result)
	})

	t.Run("EmptyString", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		ctx := WithEnvScope(context.Background(), scope)

		result := ExpandEnvContext(ctx, "")
		assert.Equal(t, "", result)
	})
}
