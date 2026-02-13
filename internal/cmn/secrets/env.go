package secrets

import (
	"context"
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/cmn/eval"
	"github.com/dagu-org/dagu/internal/core"
)

func init() {
	registerResolver("env", func(_ []string) Resolver {
		return &envResolver{}
	})
}

// envResolver reads secrets from environment variables.
// This provider is suitable for:
//   - Local development
//   - CI/CD environments where secrets are injected at runtime
//   - Testing
//
// For production, prefer external providers like GCP Secret Manager.
type envResolver struct{}

// Name returns the provider identifier.
func (r *envResolver) Name() string {
	return "env"
}

// Validate checks if the secret reference is valid for environment variables.
func (r *envResolver) Validate(ref core.SecretRef) error {
	if ref.Key == "" {
		return fmt.Errorf("key (environment variable name) is required")
	}
	return nil
}

// Resolve fetches the secret value from the environment.
// It first checks the context-provided EnvScope (for DAG-level env vars),
// then falls back to the global OS environment.
func (r *envResolver) Resolve(ctx context.Context, ref core.SecretRef) (string, error) {
	// First check context-provided env vars (DAG env: field, .env files)
	if scope := eval.GetEnvScope(ctx); scope != nil {
		if value, exists := scope.Get(ref.Key); exists {
			return value, nil
		}
	}

	// Fall back to global OS environment
	value, exists := os.LookupEnv(ref.Key)
	if !exists {
		return "", fmt.Errorf("environment variable %q is not set", ref.Key)
	}
	return value, nil
}

// CheckAccessibility verifies the environment variable exists without reading its value.
// It first checks the context-provided EnvScope, then falls back to the global OS environment.
func (r *envResolver) CheckAccessibility(ctx context.Context, ref core.SecretRef) error {
	// First check context-provided env vars (DAG env: field, .env files)
	if scope := eval.GetEnvScope(ctx); scope != nil {
		if _, exists := scope.Get(ref.Key); exists {
			return nil
		}
	}

	// Fall back to global OS environment
	_, exists := os.LookupEnv(ref.Key)
	if !exists {
		return fmt.Errorf("environment variable %q is not set", ref.Key)
	}
	return nil
}
