package secrets

import (
	"context"
	"fmt"
	"sync"

	"github.com/dagu-org/dagu/internal/core"
)

// Resolver fetches secret values from a specific backend.
// Implementations must be thread-safe as they may be called concurrently.
type Resolver interface {
	// Name returns the provider identifier (e.g., "env", "file", "gcp-secrets").
	Name() string

	// Resolve fetches the secret value for the given reference.
	// Returns an error if the secret cannot be retrieved.
	Resolve(ctx context.Context, ref core.SecretRef) (string, error)

	// Validate checks if the secret reference is structurally valid for this provider.
	// This is called at parse time and should not make network calls.
	Validate(ref core.SecretRef) error

	// CheckAccessibility verifies the secret is accessible without fetching its value.
	// Used during dry-run and validate modes.
	// Should verify:
	//   - Provider is reachable
	//   - Credentials are valid
	//   - Secret exists
	//   - Caller has permission
	CheckAccessibility(ctx context.Context, ref core.SecretRef) error
}

// Registry manages all secret resolvers.
// It is thread-safe and can be used concurrently.
type Registry struct {
	resolvers  map[string]Resolver
	mu         sync.RWMutex
	workingDir string
}

var (
	// globalResolvers stores resolver factories that are registered via init()
	globalResolvers = make(map[string]func(workingDir string) Resolver)
	globalMu        sync.RWMutex
)

// registerResolver adds a resolver factory to be used by all registries.
// This is called from init() functions in resolver implementation files.
func registerResolver(name string, factory func(workingDir string) Resolver) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalResolvers[name] = factory
}

// NewRegistry creates a new registry with all registered providers.
// workingDir is used by the file provider to resolve relative paths.
func NewRegistry(workingDir string) *Registry {
	globalMu.RLock()
	defer globalMu.RUnlock()

	r := &Registry{
		resolvers:  make(map[string]Resolver),
		workingDir: workingDir,
	}

	// Instantiate all registered providers
	for name, factory := range globalResolvers {
		r.resolvers[name] = factory(workingDir)
	}

	return r
}

// Register adds a custom resolver to the registry.
// If a resolver with the same name already exists, it will be replaced.
// This is useful for adding custom providers or testing.
func (r *Registry) Register(name string, res Resolver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resolvers[name] = res
}

// Get retrieves a resolver by provider name.
// Returns nil if the provider is not registered.
func (r *Registry) Get(provider string) Resolver {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.resolvers[provider]
}

// Resolve fetches a single secret value.
// Returns an error if the provider is unknown or resolution fails.
func (r *Registry) Resolve(ctx context.Context, ref core.SecretRef) (string, error) {
	if ref.Provider == "" {
		return "", fmt.Errorf("provider is required for secret %q", ref.Name)
	}

	res := r.Get(ref.Provider)
	if res == nil {
		return "", fmt.Errorf("unknown secret provider: %s", ref.Provider)
	}

	if err := res.Validate(ref); err != nil {
		return "", fmt.Errorf("invalid secret reference for %q: %w", ref.Name, err)
	}

	value, err := res.Resolve(ctx, ref)
	if err != nil {
		return "", fmt.Errorf("failed to resolve secret %q from provider %q: %w", ref.Name, ref.Provider, err)
	}

	return value, nil
}

// ResolveAll fetches all secrets and returns them as environment variable strings.
// Format: "NAME=value"
// Returns an error if any secret fails to resolve.
func (r *Registry) ResolveAll(ctx context.Context, refs []core.SecretRef) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}

	envVars := make([]string, 0, len(refs))

	for _, ref := range refs {
		value, err := r.Resolve(ctx, ref)
		if err != nil {
			return nil, err
		}
		envVars = append(envVars, fmt.Sprintf("%s=%s", ref.Name, value))
	}

	return envVars, nil
}

// CheckAccessibility validates that all secrets are accessible without fetching values.
// Used during dry-run and validate modes.
// Returns an error if any secret is inaccessible.
func (r *Registry) CheckAccessibility(ctx context.Context, refs []core.SecretRef) error {
	if len(refs) == 0 {
		return nil
	}

	for _, ref := range refs {
		if ref.Provider == "" {
			return fmt.Errorf("provider is required for secret %q", ref.Name)
		}

		res := r.Get(ref.Provider)
		if res == nil {
			return fmt.Errorf("unknown secret provider: %s", ref.Provider)
		}

		if err := res.Validate(ref); err != nil {
			return fmt.Errorf("invalid secret reference for %q: %w", ref.Name, err)
		}

		if err := res.CheckAccessibility(ctx, ref); err != nil {
			return fmt.Errorf("secret %q is not accessible: %w", ref.Name, err)
		}
	}

	return nil
}

// Providers returns the names of all registered providers.
func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.resolvers))
	for name := range r.resolvers {
		names = append(names, name)
	}
	return names
}
