package agent

import "context"

// Context keys for agent stores.
// These allow agent stores to be injected into Go contexts without
// creating a backwards dependency from the execution context to the agent package.

type configStoreKey struct{}
type modelStoreKey struct{}
type memoryStoreKey struct{}

// WithConfigStore injects a ConfigStore into the context.
func WithConfigStore(ctx context.Context, s ConfigStore) context.Context {
	return context.WithValue(ctx, configStoreKey{}, s)
}

// GetConfigStore retrieves a ConfigStore from the context.
// Returns nil if no ConfigStore is set.
func GetConfigStore(ctx context.Context) ConfigStore {
	if v := ctx.Value(configStoreKey{}); v != nil {
		if s, ok := v.(ConfigStore); ok {
			return s
		}
	}
	return nil
}

// WithModelStore injects a ModelStore into the context.
func WithModelStore(ctx context.Context, s ModelStore) context.Context {
	return context.WithValue(ctx, modelStoreKey{}, s)
}

// GetModelStore retrieves a ModelStore from the context.
// Returns nil if no ModelStore is set.
func GetModelStore(ctx context.Context) ModelStore {
	if v := ctx.Value(modelStoreKey{}); v != nil {
		if s, ok := v.(ModelStore); ok {
			return s
		}
	}
	return nil
}

// WithMemoryStore injects a MemoryStore into the context.
func WithMemoryStore(ctx context.Context, s MemoryStore) context.Context {
	return context.WithValue(ctx, memoryStoreKey{}, s)
}

// GetMemoryStore retrieves a MemoryStore from the context.
// Returns nil if no MemoryStore is set.
func GetMemoryStore(ctx context.Context) MemoryStore {
	if v := ctx.Value(memoryStoreKey{}); v != nil {
		if s, ok := v.(MemoryStore); ok {
			return s
		}
	}
	return nil
}
