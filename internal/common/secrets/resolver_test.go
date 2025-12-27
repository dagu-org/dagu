package secrets

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry("/tmp")
	require.NotNil(t, registry)

	// Should have built-in providers registered
	providers := registry.Providers()
	assert.Contains(t, providers, "env")
	assert.Contains(t, providers, "file")
	assert.Len(t, providers, 2)
}

func TestRegistry_Register(t *testing.T) {
	registry := NewRegistry("/tmp")

	// Create a mock resolver
	mock := &mockResolver{mockName: "mock"}
	registry.Register("mock", mock)

	// Should be retrievable
	resolver := registry.Get("mock")
	require.NotNil(t, resolver)
	assert.Equal(t, "mock", resolver.Name())
}

func TestRegistry_Get(t *testing.T) {
	registry := NewRegistry("/tmp")

	t.Run("ExistingProvider", func(t *testing.T) {
		resolver := registry.Get("env")
		require.NotNil(t, resolver)
		assert.Equal(t, "env", resolver.Name())
	})

	t.Run("NonExistentProvider", func(t *testing.T) {
		resolver := registry.Get("nonexistent")
		assert.Nil(t, resolver)
	})
}

func TestRegistry_Resolve(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")

	// Set up environment variable for testing
	t.Setenv("TEST_SECRET", "test_value")

	t.Run("SuccessfulResolve", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "TEST_SECRET",
			Provider: "env",
			Key:      "TEST_SECRET",
		}

		value, err := registry.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "test_value", value)
	})

	t.Run("UnknownProvider", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "unknown",
			Key:      "key",
		}

		_, err := registry.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown secret provider: unknown")
	})

	t.Run("EmptyProvider", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "",
			Key:      "key",
		}

		_, err := registry.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provider is required")
	})

	t.Run("InvalidReference", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "env",
			Key:      "", // Empty key
		}

		_, err := registry.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid secret reference")
	})

	t.Run("ResolutionFailure", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "NONEXISTENT",
			Provider: "env",
			Key:      "NONEXISTENT_VAR",
		}

		_, err := registry.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to resolve secret")
		assert.Contains(t, err.Error(), "NONEXISTENT")
	})
}

func TestRegistry_ResolveAll(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")

	t.Run("MultipleSecrets", func(t *testing.T) {
		t.Setenv("SECRET1", "value1")
		t.Setenv("SECRET2", "value2")

		refs := []core.SecretRef{
			{Name: "SECRET1", Provider: "env", Key: "SECRET1"},
			{Name: "SECRET2", Provider: "env", Key: "SECRET2"},
		}

		envVars, err := registry.ResolveAll(ctx, refs)
		require.NoError(t, err)
		assert.Len(t, envVars, 2)
		assert.Contains(t, envVars, "SECRET1=value1")
		assert.Contains(t, envVars, "SECRET2=value2")
	})

	t.Run("EmptyList", func(t *testing.T) {
		envVars, err := registry.ResolveAll(ctx, nil)
		require.NoError(t, err)
		assert.Nil(t, envVars)
	})

	t.Run("OneFailsAll", func(t *testing.T) {
		t.Setenv("SECRET1", "value1")

		refs := []core.SecretRef{
			{Name: "SECRET1", Provider: "env", Key: "SECRET1"},
			{Name: "MISSING", Provider: "env", Key: "MISSING_VAR"},
		}

		_, err := registry.ResolveAll(ctx, refs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MISSING")
	})
}

func TestRegistry_CheckAccessibility(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry("/tmp")

	t.Run("AllAccessible", func(t *testing.T) {
		t.Setenv("SECRET1", "value1")
		t.Setenv("SECRET2", "value2")

		refs := []core.SecretRef{
			{Name: "SECRET1", Provider: "env", Key: "SECRET1"},
			{Name: "SECRET2", Provider: "env", Key: "SECRET2"},
		}

		err := registry.CheckAccessibility(ctx, refs)
		require.NoError(t, err)
	})

	t.Run("OneInaccessible", func(t *testing.T) {
		t.Setenv("SECRET1", "value1")

		refs := []core.SecretRef{
			{Name: "SECRET1", Provider: "env", Key: "SECRET1"},
			{Name: "MISSING", Provider: "env", Key: "MISSING_VAR"},
		}

		err := registry.CheckAccessibility(ctx, refs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "MISSING")
		assert.Contains(t, err.Error(), "not accessible")
	})

	t.Run("EmptyList", func(t *testing.T) {
		err := registry.CheckAccessibility(ctx, nil)
		require.NoError(t, err)
	})

	t.Run("UnknownProvider", func(t *testing.T) {
		refs := []core.SecretRef{
			{Name: "SECRET", Provider: "unknown", Key: "key"},
		}

		err := registry.CheckAccessibility(ctx, refs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown secret provider")
	})
}

func TestRegistry_Providers(t *testing.T) {
	registry := NewRegistry("/tmp")

	providers := registry.Providers()
	assert.Len(t, providers, 2)
	assert.Contains(t, providers, "env")
	assert.Contains(t, providers, "file")

	// Add custom provider
	mock := &mockResolver{mockName: "custom"}
	registry.Register("custom", mock)

	providers = registry.Providers()
	assert.Len(t, providers, 3)
	assert.Contains(t, providers, "custom")
}

var _ Resolver = (*mockResolver)(nil)

// mockResolver is a test double for the Resolver interface
type mockResolver struct {
	mockName        string
	resolveFunc     func(context.Context, core.SecretRef) (string, error)
	validateFunc    func(core.SecretRef) error
	checkAccessFunc func(context.Context, core.SecretRef) error
}

func (m *mockResolver) Name() string {
	return m.mockName
}

func (m *mockResolver) Resolve(ctx context.Context, ref core.SecretRef) (string, error) {
	if m.resolveFunc != nil {
		return m.resolveFunc(ctx, ref)
	}
	return "mock_value", nil
}

func (m *mockResolver) Validate(ref core.SecretRef) error {
	if m.validateFunc != nil {
		return m.validateFunc(ref)
	}
	return nil
}

func (m *mockResolver) CheckAccessibility(ctx context.Context, ref core.SecretRef) error {
	if m.checkAccessFunc != nil {
		return m.checkAccessFunc(ctx, ref)
	}
	return nil
}
