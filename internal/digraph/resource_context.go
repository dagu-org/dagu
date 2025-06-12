package digraph

import (
	"context"
)

// resourceControllerKey is a context key for the resource controller
type resourceControllerKey struct{}

// WithResourceController returns a new context with the resource controller
// We use any to avoid circular dependency with resource package
func WithResourceController(ctx context.Context, rc any) context.Context {
	return context.WithValue(ctx, resourceControllerKey{}, rc)
}

// GetResourceController returns the resource controller from the context
// Returns nil if not found. Caller should type assert to *resource.ResourceController
func GetResourceController(ctx context.Context) any {
	return ctx.Value(resourceControllerKey{})
}
