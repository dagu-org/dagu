package cmdutil

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

func TestExpandEnvContextSkipOS(t *testing.T) {
	t.Run("WithScopeUserVarExpanded", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("MY_VAR", "scope_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContextSkipOS(ctx, "Value is $MY_VAR")
		assert.Equal(t, "Value is scope_value", result)
	})

	t.Run("WithScopeOSVarNotExpanded", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("OS_VAR", "os_value", EnvSourceOS)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContextSkipOS(ctx, "Value is $OS_VAR")
		assert.Equal(t, "Value is $OS_VAR", result)
	})

	t.Run("MixedSourcesOnlyUserExpanded", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("USER_VAR", "user", EnvSourceDAGEnv).
			WithEntry("OS_VAR", "os", EnvSourceOS).
			WithEntry("SECRET_VAR", "secret", EnvSourceSecret)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContextSkipOS(ctx, "$USER_VAR $OS_VAR $SECRET_VAR")
		assert.Equal(t, "user $OS_VAR secret", result)
	})

	t.Run("WithoutScopePreservesAll", func(t *testing.T) {
		ctx := context.Background()
		result := ExpandEnvContextSkipOS(ctx, "Value is $SOME_VAR")
		assert.Equal(t, "Value is $SOME_VAR", result)
	})

	t.Run("NilContextPreservesAll", func(t *testing.T) {
		result := ExpandEnvContextSkipOS(context.TODO(), "Value is $SOME_VAR")
		assert.Equal(t, "Value is $SOME_VAR", result)
	})

	t.Run("BracedVariableSyntax", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("USER_VAR", "user", EnvSourceDAGEnv).
			WithEntry("OS_VAR", "os", EnvSourceOS)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContextSkipOS(ctx, "${USER_VAR}/${OS_VAR}")
		assert.Equal(t, "user/${OS_VAR}", result)
	})

	t.Run("AllNonOSSourcesExpanded", func(t *testing.T) {
		scope := NewEnvScope(nil, false).
			WithEntry("DAG_VAR", "dag", EnvSourceDAGEnv).
			WithEntry("DOTENV_VAR", "dotenv", EnvSourceDotEnv).
			WithEntry("PARAM_VAR", "param", EnvSourceParam).
			WithEntry("OUTPUT_VAR", "output", EnvSourceOutput).
			WithEntry("SECRET_VAR", "secret", EnvSourceSecret).
			WithEntry("STEP_VAR", "step", EnvSourceStepEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := ExpandEnvContextSkipOS(ctx, "$DAG_VAR $DOTENV_VAR $PARAM_VAR $OUTPUT_VAR $SECRET_VAR $STEP_VAR")
		assert.Equal(t, "dag dotenv param output secret step", result)
	})
}
