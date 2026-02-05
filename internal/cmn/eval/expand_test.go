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

// --- expandWithShellContext coverage ---

func TestExpandWithShellContext_ShellDisabledEnvEnabled(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandShell = false
	opts.ExpandEnv = true
	opts.ExpandOS = true
	t.Setenv("MYVAR", "myval")

	result, err := expandWithShellContext(ctx, "$MYVAR", opts)
	require.NoError(t, err)
	assert.Equal(t, "myval", result)
}

func TestExpandWithShellContext_ShellDisabledEnvDisabled(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandShell = false
	opts.ExpandEnv = false

	result, err := expandWithShellContext(ctx, "$MYVAR", opts)
	require.NoError(t, err)
	assert.Equal(t, "$MYVAR", result)
}

func TestExpandWithShellContext_EmptyInput(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := expandWithShellContext(ctx, "", opts)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestExpandWithShellContext_UnexpectedCommand(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandOS = true
	t.Setenv("KEEP", "kept")

	// $(command) triggers UnexpectedCommandError and falls back to ExpandEnvContext
	result, err := expandWithShellContext(ctx, "$(echo x) $KEEP", opts)
	require.NoError(t, err)
	assert.Contains(t, result, "kept")
}

// --- expandEnvScopeOnly tests ---

func TestExpandEnvScopeOnly_WithScope(t *testing.T) {
	scope := NewEnvScope(nil, false).
		WithEntry("DAG_VAR", "dag_value", EnvSourceDAGEnv)
	ctx := WithEnvScope(context.Background(), scope)

	result := expandEnvScopeOnly(ctx, "Value is $DAG_VAR")
	assert.Equal(t, "Value is dag_value", result)
}

func TestExpandEnvScopeOnly_NilScope(t *testing.T) {
	ctx := context.Background()
	result := expandEnvScopeOnly(ctx, "Value is $DAG_VAR")
	assert.Equal(t, "Value is $DAG_VAR", result)
}

func TestExpandEnvScopeOnly_OSEntriesSkipped(t *testing.T) {
	scope := NewEnvScope(nil, false).
		WithEntry("OS_VAR", "os_value", EnvSourceOS).
		WithEntry("DAG_VAR", "dag_value", EnvSourceDAGEnv)
	ctx := WithEnvScope(context.Background(), scope)

	result := expandEnvScopeOnly(ctx, "$OS_VAR and $DAG_VAR")
	assert.Equal(t, "$OS_VAR and dag_value", result)
}

func TestExpandWithShellContext_NonUnexpectedCommandError(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandOS = true

	// ${UNSET_VAR_ABC:?msg} triggers a non-UnexpectedCommand error from expand.Literal
	_, err := expandWithShellContext(ctx, "${UNSET_VAR_ABC:?required}", opts)
	assert.Error(t, err)
}
