package cmdutil

import (
	"context"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnvScope(t *testing.T) {
	t.Run("NoParentNoOS", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		require.NotNil(t, scope)
		assert.Empty(t, scope.entries)
		assert.Nil(t, scope.parent)
	})

	t.Run("NoParentWithOS", func(t *testing.T) {
		// Set a test environment variable
		key := "TEST_ENVSCOPE_VAR"
		value := "test_value"
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		scope := NewEnvScope(nil, true)
		require.NotNil(t, scope)
		assert.NotEmpty(t, scope.entries)

		// Verify our test variable is present
		got, ok := scope.Get(key)
		assert.True(t, ok)
		assert.Equal(t, value, got)
	})

	t.Run("WithParent", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("PARENT_VAR", "parent_value", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)
		require.NotNil(t, child)
		assert.Equal(t, parent, child.parent)

		// Child should be able to access parent's variables
		got, ok := child.Get("PARENT_VAR")
		assert.True(t, ok)
		assert.Equal(t, "parent_value", got)
	})
}

func TestEnvScope_Set(t *testing.T) {
	t.Run("NewKey", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("NEW_KEY", "new_value", EnvSourceDAGEnv)

		got, ok := scope.Get("NEW_KEY")
		assert.True(t, ok)
		assert.Equal(t, "new_value", got)
	})

	t.Run("OverrideExisting", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("KEY", "original", EnvSourceDAGEnv)
		scope.Set("KEY", "updated", EnvSourceStep)

		got, ok := scope.Get("KEY")
		assert.True(t, ok)
		assert.Equal(t, "updated", got)

		// Verify source was updated too
		entry, ok := scope.GetEntry("KEY")
		assert.True(t, ok)
		assert.Equal(t, EnvSourceStep, entry.Source)
	})

	t.Run("DifferentSources", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		sources := []EnvSource{
			EnvSourceOS,
			EnvSourceDAGEnv,
			EnvSourceDotEnv,
			EnvSourceParam,
			EnvSourceStep,
			EnvSourceSecret,
		}

		for i, src := range sources {
			key := "KEY_" + string(src)
			scope.Set(key, "value", src)

			entry, ok := scope.GetEntry(key)
			assert.True(t, ok, "source %d", i)
			assert.Equal(t, src, entry.Source, "source %d", i)
		}
	})
}

func TestEnvScope_Get(t *testing.T) {
	t.Run("KeyInCurrentScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("LOCAL_KEY", "local_value", EnvSourceDAGEnv)

		got, ok := scope.Get("LOCAL_KEY")
		assert.True(t, ok)
		assert.Equal(t, "local_value", got)
	})

	t.Run("KeyInParentScope", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("PARENT_KEY", "parent_value", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)

		got, ok := child.Get("PARENT_KEY")
		assert.True(t, ok)
		assert.Equal(t, "parent_value", got)
	})

	t.Run("KeyNotFound", func(t *testing.T) {
		scope := NewEnvScope(nil, false)

		got, ok := scope.Get("NONEXISTENT")
		assert.False(t, ok)
		assert.Empty(t, got)
	})

	t.Run("ChildOverridesParent", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("KEY", "parent_value", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)
		child.Set("KEY", "child_value", EnvSourceStep)

		got, ok := child.Get("KEY")
		assert.True(t, ok)
		assert.Equal(t, "child_value", got)

		// Parent should still have original value
		parentGot, ok := parent.Get("KEY")
		assert.True(t, ok)
		assert.Equal(t, "parent_value", parentGot)
	})
}

func TestEnvScope_GetEntry(t *testing.T) {
	t.Run("EntryInCurrentScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("KEY", "value", EnvSourceSecret)

		entry, ok := scope.GetEntry("KEY")
		assert.True(t, ok)
		assert.Equal(t, "KEY", entry.Key)
		assert.Equal(t, "value", entry.Value)
		assert.Equal(t, EnvSourceSecret, entry.Source)
	})

	t.Run("EntryInParentScope", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("PARENT_KEY", "parent_value", EnvSourceDotEnv)

		child := NewEnvScope(parent, false)

		entry, ok := child.GetEntry("PARENT_KEY")
		assert.True(t, ok)
		assert.Equal(t, "PARENT_KEY", entry.Key)
		assert.Equal(t, "parent_value", entry.Value)
		assert.Equal(t, EnvSourceDotEnv, entry.Source)
	})

	t.Run("EntryNotFound", func(t *testing.T) {
		scope := NewEnvScope(nil, false)

		entry, ok := scope.GetEntry("NONEXISTENT")
		assert.False(t, ok)
		assert.Empty(t, entry.Key)
		assert.Empty(t, entry.Value)
	})
}

func TestEnvScope_ToSlice(t *testing.T) {
	t.Run("EmptyScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		result := scope.ToSlice()
		assert.Empty(t, result)
	})

	t.Run("WithEntries", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("A", "1", EnvSourceDAGEnv)
		scope.Set("B", "2", EnvSourceDAGEnv)

		result := scope.ToSlice()
		sort.Strings(result)

		assert.Len(t, result, 2)
		assert.Contains(t, result, "A=1")
		assert.Contains(t, result, "B=2")
	})

	t.Run("WithParentOverride", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("SHARED", "parent", EnvSourceDAGEnv)
		parent.Set("PARENT_ONLY", "p", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)
		child.Set("SHARED", "child", EnvSourceStep)
		child.Set("CHILD_ONLY", "c", EnvSourceStep)

		result := child.ToSlice()

		// Convert to map for easier checking
		resultMap := make(map[string]string)
		for _, s := range result {
			if k, v, ok := strings.Cut(s, "="); ok {
				resultMap[k] = v
			}
		}

		assert.Equal(t, "child", resultMap["SHARED"], "child should override parent")
		assert.Equal(t, "p", resultMap["PARENT_ONLY"])
		assert.Equal(t, "c", resultMap["CHILD_ONLY"])
	})
}

func TestEnvScope_ToMap(t *testing.T) {
	t.Run("EmptyScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		result := scope.ToMap()
		assert.Empty(t, result)
	})

	t.Run("WithEntries", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("X", "10", EnvSourceDAGEnv)
		scope.Set("Y", "20", EnvSourceDAGEnv)

		result := scope.ToMap()
		assert.Len(t, result, 2)
		assert.Equal(t, "10", result["X"])
		assert.Equal(t, "20", result["Y"])
	})

	t.Run("WithParent", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("A", "1", EnvSourceDAGEnv)
		parent.Set("B", "2", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)
		child.Set("B", "overridden", EnvSourceStep)
		child.Set("C", "3", EnvSourceStep)

		result := child.ToMap()
		assert.Len(t, result, 3)
		assert.Equal(t, "1", result["A"])
		assert.Equal(t, "overridden", result["B"])
		assert.Equal(t, "3", result["C"])
	})
}

func TestEnvScope_Expand(t *testing.T) {
	t.Run("VariableExists", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("NAME", "World", EnvSourceDAGEnv)

		result := scope.Expand("Hello, $NAME!")
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("VariableWithBraces", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("NAME", "World", EnvSourceDAGEnv)

		result := scope.Expand("Hello, ${NAME}!")
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("VariableNotFoundReturnsEmpty", func(t *testing.T) {
		scope := NewEnvScope(nil, false)

		result := scope.Expand("Hello, $UNKNOWN!")
		assert.Equal(t, "Hello, !", result)
	})

	t.Run("MultipleVariables", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("FIRST", "Hello", EnvSourceDAGEnv)
		scope.Set("SECOND", "World", EnvSourceDAGEnv)

		result := scope.Expand("$FIRST, $SECOND!")
		assert.Equal(t, "Hello, World!", result)
	})

	t.Run("VariableFromParent", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		parent.Set("PARENT_VAR", "from_parent", EnvSourceDAGEnv)

		child := NewEnvScope(parent, false)
		child.Set("CHILD_VAR", "from_child", EnvSourceStep)

		result := child.Expand("$PARENT_VAR and $CHILD_VAR")
		assert.Equal(t, "from_parent and from_child", result)
	})

	t.Run("NoVariables", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		result := scope.Expand("plain text")
		assert.Equal(t, "plain text", result)
	})
}

func TestEnvScope_Debug(t *testing.T) {
	t.Run("EmptyScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		result := scope.Debug()
		assert.Contains(t, result, "EnvScope{")
		assert.Contains(t, result, "}")
	})

	t.Run("WithEntries", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("KEY", "value", EnvSourceDAGEnv)

		result := scope.Debug()
		assert.Contains(t, result, "EnvScope{")
		assert.Contains(t, result, "KEY")
		assert.Contains(t, result, "value")
		assert.Contains(t, result, "dag_env")
		assert.Contains(t, result, "}")
	})

	t.Run("WithParent", func(t *testing.T) {
		parent := NewEnvScope(nil, false)
		child := NewEnvScope(parent, false)

		result := child.Debug()
		assert.Contains(t, result, "parent: <yes>")
	})

	t.Run("WithoutParent", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		result := scope.Debug()
		assert.NotContains(t, result, "parent:")
	})
}

func TestWithEnvScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope.Set("TEST", "value", EnvSourceDAGEnv)

	ctx := context.Background()
	ctxWithScope := WithEnvScope(ctx, scope)

	retrieved := GetEnvScope(ctxWithScope)
	require.NotNil(t, retrieved)

	got, ok := retrieved.Get("TEST")
	assert.True(t, ok)
	assert.Equal(t, "value", got)
}

func TestGetEnvScope(t *testing.T) {
	t.Run("NilContext", func(t *testing.T) {
		result := GetEnvScope(nil)
		assert.Nil(t, result)
	})

	t.Run("ContextWithoutScope", func(t *testing.T) {
		ctx := context.Background()
		result := GetEnvScope(ctx)
		assert.Nil(t, result)
	})

	t.Run("ContextWithScope", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("KEY", "value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := GetEnvScope(ctx)

		require.NotNil(t, result)
		got, ok := result.Get("KEY")
		assert.True(t, ok)
		assert.Equal(t, "value", got)
	})
}

func TestGetEnvScopeOrOS(t *testing.T) {
	t.Run("WithScopeInContext", func(t *testing.T) {
		scope := NewEnvScope(nil, false)
		scope.Set("CUSTOM", "custom_value", EnvSourceDAGEnv)

		ctx := WithEnvScope(context.Background(), scope)
		result := GetEnvScopeOrOS(ctx)

		require.NotNil(t, result)
		// Should return the context scope, not a new one
		got, ok := result.Get("CUSTOM")
		assert.True(t, ok)
		assert.Equal(t, "custom_value", got)
	})

	t.Run("WithoutScopeInContext", func(t *testing.T) {
		// Set a test environment variable
		key := "TEST_GETENVSCOPE_VAR"
		value := "os_value"
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		ctx := context.Background()
		result := GetEnvScopeOrOS(ctx)

		require.NotNil(t, result)
		// Should return a new scope with OS environment
		got, ok := result.Get(key)
		assert.True(t, ok)
		assert.Equal(t, value, got)
	})

	t.Run("NilContext", func(t *testing.T) {
		// Set a test environment variable
		key := "TEST_GETENVSCOPE_NIL_VAR"
		value := "nil_ctx_value"
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		result := GetEnvScopeOrOS(nil)

		require.NotNil(t, result)
		// Should return a new scope with OS environment
		got, ok := result.Get(key)
		assert.True(t, ok)
		assert.Equal(t, value, got)
	})
}

func TestEnvSource_Constants(t *testing.T) {
	// Verify all source constants are defined correctly
	assert.Equal(t, EnvSource("os"), EnvSourceOS)
	assert.Equal(t, EnvSource("dag_env"), EnvSourceDAGEnv)
	assert.Equal(t, EnvSource("dotenv"), EnvSourceDotEnv)
	assert.Equal(t, EnvSource("param"), EnvSourceParam)
	assert.Equal(t, EnvSource("step"), EnvSourceStep)
	assert.Equal(t, EnvSource("secret"), EnvSourceSecret)
}
