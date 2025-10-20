package secrets

import (
	"context"
	"fmt"
	"os"

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
func (r *envResolver) Resolve(_ context.Context, ref core.SecretRef) (string, error) {
	value := os.Getenv(ref.Key)
	if value == "" {
		// Check if variable exists but is empty, or doesn't exist at all
		_, exists := os.LookupEnv(ref.Key)
		if !exists {
			return "", fmt.Errorf("environment variable %q is not set", ref.Key)
		}
		// Variable exists but is empty - this is allowed
		return "", nil
	}
	return value, nil
}

// CheckAccessibility verifies the environment variable exists without reading its value.
func (r *envResolver) CheckAccessibility(_ context.Context, ref core.SecretRef) error {
	_, exists := os.LookupEnv(ref.Key)
	if !exists {
		return fmt.Errorf("environment variable %q is not set", ref.Key)
	}
	return nil
}
