package eval

import (
	"context"
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
		t.Setenv(key, "os_value")

		scope := NewEnvScope(nil, false).
			WithEntry(key, "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContext(ctx, "Value is $"+key)
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("WithoutScopeFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_OS_FALLBACK"
		t.Setenv(key, "os_value")

		result := ExpandEnvContext(context.Background(), "Value is $"+key)
		assert.Equal(t, "Value is os_value", result)
	})

	t.Run("NilContextFallsBackToOSExpandEnv", func(t *testing.T) {
		key := "TEST_EXPAND_NIL_CTX"
		t.Setenv(key, "nil_ctx_value")

		result := ExpandEnvContext(nil, "Value is $"+key) //nolint:staticcheck // testing nil context handling
		assert.Equal(t, "Value is nil_ctx_value", result)
	})

	t.Run("VariableNotFoundPreserved", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		ctx := WithEnvScope(context.Background(), scope)

		assert.Equal(t, "Hello $NONEXISTENT!", ExpandEnvContext(ctx, "Hello $NONEXISTENT!"))
		assert.Equal(t, "Hello ${NONEXISTENT}!", ExpandEnvContext(ctx, "Hello ${NONEXISTENT}!"))
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

func TestExpandWithShellContext(t *testing.T) {
	t.Run("ShellDisabledEnvEnabled", func(t *testing.T) {
		t.Setenv("MYVAR", "myval")

		opts := NewOptions()
		opts.ExpandShell = false
		opts.ExpandEnv = true
		opts.ExpandOS = true

		result, err := expandWithShellContext(context.Background(), "$MYVAR", opts)
		require.NoError(t, err)
		assert.Equal(t, "myval", result)
	})

	t.Run("ShellDisabledEnvDisabled", func(t *testing.T) {
		opts := NewOptions()
		opts.ExpandShell = false
		opts.ExpandEnv = false

		result, err := expandWithShellContext(context.Background(), "$MYVAR", opts)
		require.NoError(t, err)
		assert.Equal(t, "$MYVAR", result)
	})

	t.Run("EmptyInput", func(t *testing.T) {
		result, err := expandWithShellContext(context.Background(), "", NewOptions())
		require.NoError(t, err)
		assert.Equal(t, "", result)
	})

	t.Run("UnexpectedCommandFallback", func(t *testing.T) {
		t.Setenv("KEEP", "kept")

		opts := NewOptions()
		opts.ExpandOS = true

		// $(command) triggers UnexpectedCommandError and falls back to ExpandEnvContext
		result, err := expandWithShellContext(context.Background(), "$(echo x) $KEEP", opts)
		require.NoError(t, err)
		assert.Contains(t, result, "kept")
	})

	t.Run("ParseError", func(t *testing.T) {
		opts := NewOptions()
		opts.ExpandOS = true

		_, err := expandWithShellContext(context.Background(), "${", opts)
		assert.Error(t, err)
	})

	t.Run("NonUnexpectedCommandError", func(t *testing.T) {
		opts := NewOptions()
		opts.ExpandOS = true

		// ${UNSET_VAR_ABC:?msg} triggers a non-UnexpectedCommand error from expand.Literal
		_, err := expandWithShellContext(context.Background(), "${UNSET_VAR_ABC:?required}", opts)
		assert.Error(t, err)
	})
}

func TestExpandEnvScopeOnly(t *testing.T) {
	t.Run("WithScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("DAG_VAR", "dag_value", EnvSourceDAGEnv)
		ctx := WithEnvScope(context.Background(), scope)

		result := expandEnvScopeOnly(ctx, "Value is $DAG_VAR")
		assert.Equal(t, "Value is dag_value", result)
	})

	t.Run("NilScope", func(t *testing.T) {
		result := expandEnvScopeOnly(context.Background(), "Value is $DAG_VAR")
		assert.Equal(t, "Value is $DAG_VAR", result)
	})

	t.Run("OSEntriesSkipped", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("OS_VAR", "os_value", EnvSourceOS).
			WithEntry("DAG_VAR", "dag_value", EnvSourceDAGEnv)
		ctx := WithEnvScope(context.Background(), scope)

		result := expandEnvScopeOnly(ctx, "$OS_VAR and $DAG_VAR")
		assert.Equal(t, "$OS_VAR and dag_value", result)
	})
}
