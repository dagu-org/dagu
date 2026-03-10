package secrets

import (
	"context"
	"fmt"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockVaultClient is a mock for vaultClient interface.
type MockVaultClient struct {
	ReadFunc       func(ctx context.Context, path string) (map[string]any, error)
	LookupSelfFunc func(ctx context.Context) (map[string]any, error)
}

func (m *MockVaultClient) Read(ctx context.Context, path string) (map[string]any, error) {
	if m.ReadFunc == nil {
		return nil, nil
	}
	return m.ReadFunc(ctx, path)
}

func (m *MockVaultClient) LookupSelf(ctx context.Context) (map[string]any, error) {
	if m.LookupSelfFunc == nil {
		return map[string]any{"display_name": "mock", "policies": []string{"default"}}, nil
	}
	return m.LookupSelfFunc(ctx)
}

func TestVaultResolver_Name(t *testing.T) {
	resolver := &vaultResolver{}
	assert.Equal(t, "vault", resolver.Name())
}

func TestVaultResolver_Validate(t *testing.T) {
	resolver := &vaultResolver{}

	t.Run("ValidReference", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "API_KEY",
			Provider: "vault",
			Key:      "secret/data/my-secret",
		}
		err := resolver.Validate(ref)
		require.NoError(t, err)
	})

	t.Run("EmptyKey", func(t *testing.T) {
		ref := core.SecretRef{
			Name:     "SECRET",
			Provider: "vault",
			Key:      "",
		}
		err := resolver.Validate(ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "key")
	})
}

func TestVaultResolver_Resolve(t *testing.T) {
	ctx := context.Background()

	t.Run("SuccessfulResolutionWithConvention", func(t *testing.T) {
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				if path == "kv/data/dummy" {
					return map[string]any{
						"data": map[string]any{
							"my-secret": "super-secret-value",
						},
					}, nil
				}
				return nil, fmt.Errorf("not found")
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "MY_API_KEY",
			Provider: "vault",
			Key:      "kv/data/dummy/my-secret",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "super-secret-value", value)
	})

	t.Run("TrailingSlashHandling", func(t *testing.T) {
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				if path == "kv/data/dummy" {
					return map[string]any{
						"my-secret": "value-with-slash",
					}, nil
				}
				return nil, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "MY_API_KEY",
			Provider: "vault",
			Key:      "kv/data/dummy/my-secret/",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "value-with-slash", value)
	})

	t.Run("ExplicitFieldOption", func(t *testing.T) {
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				if path == "kv/data/dummy" {
					return map[string]any{
						"api_key": "v2-secret",
					}, nil
				}
				return nil, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "MY_API_KEY",
			Provider: "vault",
			Key:      "kv/data/dummy",
			Options:  map[string]string{"field": "api_key"},
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "v2-secret", value)
	})

	t.Run("NotFound", func(t *testing.T) {
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				return nil, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "MY_SECRET",
			Provider: "vault",
			Key:      "kv/data/dummy/missing",
		}

		_, err := resolver.Resolve(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("KV_V1_DataFieldAmbiguity", func(t *testing.T) {
		// Verify that if a KV v1 secret has a field named "data" that is NOT a map,
		// we don't try to unwrap it and instead treat it as a regular field.
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				return map[string]any{
					"data": "actual-value",
				}, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "TEST",
			Provider: "vault",
			Key:      "path/data", // Convention: path="path", field="data"
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "actual-value", value)
	})

	t.Run("DeepPathConvention", func(t *testing.T) {
		mockClient := &MockVaultClient{
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				if path == "projects/team-a/production/db" {
					return map[string]any{"password": "deep-secret"}, nil
				}
				return nil, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "DB_PASS",
			Provider: "vault",
			Key:      "projects/team-a/production/db/password",
		}

		value, err := resolver.Resolve(ctx, ref)
		require.NoError(t, err)
		assert.Equal(t, "deep-secret", value)
	})
}

func TestVaultResolver_Concurrency(t *testing.T) {
	ctx := context.Background()
	mockClient := &MockVaultClient{
		ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
			return map[string]any{"value": "ok"}, nil
		},
	}

	resolver := &vaultResolver{client: mockClient}
	ref := core.SecretRef{
		Name:     "CONCURRENT",
		Provider: "vault",
		Key:      "path/value",
	}

	const numRoutines = 50
	done := make(chan bool, numRoutines)

	for range numRoutines {
		go func() {
			_, err := resolver.Resolve(ctx, ref)
			assert.NoError(t, err)
			done <- true
		}()
	}

	for range numRoutines {
		<-done
	}
}

func TestVaultResolver_CheckAccessibility(t *testing.T) {
	ctx := context.Background()

	t.Run("Accessible", func(t *testing.T) {
		mockClient := &MockVaultClient{
			LookupSelfFunc: func(ctx context.Context) (map[string]any, error) {
				return map[string]any{"id": "token"}, nil
			},
			ReadFunc: func(ctx context.Context, path string) (map[string]any, error) {
				return map[string]any{"field": "exists"}, nil
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "TEST",
			Provider: "vault",
			Key:      "path/field",
		}

		err := resolver.CheckAccessibility(ctx, ref)
		require.NoError(t, err)
	})

	t.Run("TokenInvalid", func(t *testing.T) {
		mockClient := &MockVaultClient{
			LookupSelfFunc: func(ctx context.Context) (map[string]any, error) {
				return nil, fmt.Errorf("permission denied")
			},
		}

		resolver := &vaultResolver{client: mockClient}
		ref := core.SecretRef{
			Name:     "TEST",
			Provider: "vault",
			Key:      "path/field",
		}

		err := resolver.CheckAccessibility(ctx, ref)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token lookup failed")
	})
}
