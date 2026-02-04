package eval

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExpandEnvContext(t *testing.T) {
	t.Run("WithScopeInContext", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("MY_VAR", "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "Value is $MY_VAR")
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("ScopeTakesPrecedenceOverOSEnv", func(t *testing.T) {
		key := "TEST_EXPAND_PRECEDENCE"
		require.NoError(t, os.Setenv(key, "os_value"))
		defer func() { _ = os.Unsetenv(key) }()

		scope := NewEnvScope(nil, false).
			WithEntry(key, "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "Value is $"+key)
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("WithoutScopeFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_OS_FALLBACK"
		require.NoError(t, os.Setenv(key, "os_value"))
		defer func() { _ = os.Unsetenv(key) }()

		ctx := context.Background()
		result := ExpandEnvContext(ctx, "Value is $"+key)
		assert.Equal(t, "Value is os_value", result)
	})

	t.Run("NilContextFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_NIL_CTX"
		require.NoError(t, os.Setenv(key, "nil_ctx_value"))
		defer func() { _ = os.Unsetenv(key) }()

		result := ExpandEnvContext(nil, "Value is $"+key) //nolint:staticcheck // testing nil context handling
		assert.Equal(t, "Value is nil_ctx_value", result)
	})

	t.Run("VariableNotFoundPreserved", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		ctx := WithEnvScope(context.Background(), scope)

		// $VAR syntax preserved
		result := ExpandEnvContext(ctx, "Hello $NONEXISTENT!")
		assert.Equal(t, "Hello $NONEXISTENT!", result)

		// ${VAR} syntax preserved
		result = ExpandEnvContext(ctx, "Hello ${NONEXISTENT}!")
		assert.Equal(t, "Hello ${NONEXISTENT}!", result)
	})

	t.Run("BracedVariableSyntax", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("VAR", "value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "${VAR}Suffix")
		assert.Equal(t, "valueSuffix", result)
	})

	t.Run("MultipleVariables", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("FIRST", "Hello", EnvSourceDAGEnv).
			WithEntry("SECOND", "World", EnvSourceDAGEnv)

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
