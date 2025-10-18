package secrets

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvResolver_Name(t *testing.T) {
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)
	assert.Equal(t, "env", resolver.Name())
}

func TestEnvResolver_Validate(t *testing.T) {
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)

	t.Run("ValidReference", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "DB_PASSWORD",
			Provider: "env",
			Key:      "DATABASE_PASSWORD",
		}
		err := resolver.Validate(ref)
		require.NoError(t, err)
	})

	t.Run("EmptyKey", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "env",
			Key:      "",
		}
		err := resolver.Validate(ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key")
		assert.Contains(t, err.Error(), "required")
	})
}

func TestEnvResolver_Resolve(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)

	t.Run("ExistingVariable", func(t *testing.T) {
		t.Setenv("TEST_SECRET_VAR", "secret_value")

		ref := core.SecretRef{
			Name:     "DB_PASSWORD",
			Provider: "env",
			Key:      "TEST_SECRET_VAR",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "secret_value", value)
	})

	t.Run("NonExistentVariable", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "MISSING_SECRET",
			Provider: "env",
			Key:      "NONEXISTENT_VAR_12345",
		}

		_, err := resolver.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "NONEXISTENT_VAR_12345")
		assert.Contains(t, err.Error(), "not set")
	})

	t.Run("EmptyButExistingVariable", func(t *testing.T) {
		t.Setenv("EMPTY_VAR", "")

		ref := core.SecretRef{
			Name:     "EMPTY_SECRET",
			Provider: "env",
			Key:      "EMPTY_VAR",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "", value)
	})

	t.Run("VariableWithWhitespace", func(t *testing.T) {
		t.Setenv("WHITESPACE_VAR", "  value with spaces  ")

		ref := core.SecretRef{
			Name:     "WHITESPACE_SECRET",
			Provider: "env",
			Key:      "WHITESPACE_VAR",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "  value with spaces  ", value, "should NOT trim whitespace")
	})

	t.Run("MultilineVariable", func(t *testing.T) {
		multilineValue := "line1\nline2\nline3"
		t.Setenv("MULTILINE_VAR", multilineValue)

		ref := core.SecretRef{
			Name:     "MULTILINE_SECRET",
			Provider: "env",
			Key:      "MULTILINE_VAR",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, multilineValue, value)
	})

	t.Run("SpecialCharactersInValue", func(t *testing.T) {
		specialValue := "p@ssw0rd!#$%^&*()"
		t.Setenv("SPECIAL_VAR", specialValue)

		ref := core.SecretRef{
			Name:     "SPECIAL_SECRET",
			Provider: "env",
			Key:      "SPECIAL_VAR",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, specialValue, value)
	})
}

func TestEnvResolver_CheckAccessibility(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)

	t.Run("AccessibleVariable", func(t *testing.T) {
		t.Setenv("ACCESSIBLE_VAR", "value")

		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "env",
			Key:      "ACCESSIBLE_VAR",
		}

		err := resolver.CheckAccessibility(ctx, ref)
		require.NoError(t, err)
	})

	t.Run("InaccessibleVariable", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "MISSING",
			Provider: "env",
			Key:      "INACCESSIBLE_VAR_98765",
		}

		err := resolver.CheckAccessibility(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "INACCESSIBLE_VAR_98765")
		assert.Contains(t, err.Error(), "not set")
	})

	t.Run("EmptyVariableIsAccessible", func(t *testing.T) {
		t.Setenv("EMPTY_BUT_SET", "")

		ref := core.SecretRef{
			Name:     "EMPTY",
			Provider: "env",
			Key:      "EMPTY_BUT_SET",
		}

		err := resolver.CheckAccessibility(ctx, ref)
		require.NoError(t, err, "empty variables should be considered accessible")
	})
}

func TestEnvResolver_OptionsIgnored(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)

	t.Setenv("TEST_VAR", "value")

	// Options should be ignored by env resolver
	ref := core.SecretRef{
		Name:     "SECRET",
		Provider: "env",
		Key:      "TEST_VAR",
		Options: map[string]string{
			"trim":    "true",
			"custom":  "option",
			"unknown": "value",
		},
	}

	value, err := resolver.Resolve(ctx, ref)
	require.NoError(t, err)
	assert.Equal(t, "value", value, "options should not affect env resolver")
}

func TestEnvResolver_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")
	resolver := registry.Get("env")
	require.NotNil(t, resolver)

	t.Setenv("CONCURRENT_VAR", "concurrent_value")

	ref := core.SecretRef{
		Name:     "CONCURRENT",
		Provider: "env",
		Key:      "CONCURRENT_VAR",
	}

	// Run multiple goroutines concurrently
	const numGoroutines = 100
	done := make(chan bool, numGoroutines)
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			value, err := resolver.Resolve(ctx, ref)
			if err != nil {
				errors <- err
			} else if value != "concurrent_value" {
				errors <- assert.AnError
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	close(errors)
	for err := range errors {
		t.Errorf("Concurrent access error: %v", err)
	}
}
