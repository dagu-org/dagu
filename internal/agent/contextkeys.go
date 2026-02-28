package agent

import "context"

// Context keys for agent stores.
// These allow agent stores to be injected into Go contexts without
// creating a backwards dependency from the execution context to the agent package.

type configStoreKey struct{}
type modelStoreKey struct{}
type memoryStoreKey struct{}
type skillStoreKey struct{}
type soulStoreKey struct{}
type remoteNodeResolverKey struct{}

// WithConfigStore injects a ConfigStore into the context.
func WithConfigStore(ctx context.Context, s ConfigStore) context.Context {
	return context.WithValue(ctx, configStoreKey{}, s)
}

// GetConfigStore retrieves a ConfigStore from the context.
// Returns nil if no ConfigStore is set.
func GetConfigStore(ctx context.Context) ConfigStore {
	s, _ := ctx.Value(configStoreKey{}).(ConfigStore)
	return s
}

// WithModelStore injects a ModelStore into the context.
func WithModelStore(ctx context.Context, s ModelStore) context.Context {
	return context.WithValue(ctx, modelStoreKey{}, s)
}

// GetModelStore retrieves a ModelStore from the context.
// Returns nil if no ModelStore is set.
func GetModelStore(ctx context.Context) ModelStore {
	s, _ := ctx.Value(modelStoreKey{}).(ModelStore)
	return s
}

// WithMemoryStore injects a MemoryStore into the context.
func WithMemoryStore(ctx context.Context, s MemoryStore) context.Context {
	return context.WithValue(ctx, memoryStoreKey{}, s)
}

// GetMemoryStore retrieves a MemoryStore from the context.
// Returns nil if no MemoryStore is set.
func GetMemoryStore(ctx context.Context) MemoryStore {
	s, _ := ctx.Value(memoryStoreKey{}).(MemoryStore)
	return s
}

// WithSkillStore injects a SkillStore into the context.
func WithSkillStore(ctx context.Context, s SkillStore) context.Context {
	return context.WithValue(ctx, skillStoreKey{}, s)
}

// GetSkillStore retrieves a SkillStore from the context.
// Returns nil if no SkillStore is set.
func GetSkillStore(ctx context.Context) SkillStore {
	s, _ := ctx.Value(skillStoreKey{}).(SkillStore)
	return s
}

// WithSoulStore injects a SoulStore into the context.
func WithSoulStore(ctx context.Context, s SoulStore) context.Context {
	return context.WithValue(ctx, soulStoreKey{}, s)
}

// GetSoulStore retrieves a SoulStore from the context.
// Returns nil if no SoulStore is set.
func GetSoulStore(ctx context.Context) SoulStore {
	s, _ := ctx.Value(soulStoreKey{}).(SoulStore)
	return s
}

// WithRemoteNodeResolver injects a RemoteNodeResolver into the context.
func WithRemoteNodeResolver(ctx context.Context, r RemoteNodeResolver) context.Context {
	return context.WithValue(ctx, remoteNodeResolverKey{}, r)
}

// GetRemoteNodeResolver retrieves a RemoteNodeResolver from the context.
// Returns nil if no RemoteNodeResolver is set.
func GetRemoteNodeResolver(ctx context.Context) RemoteNodeResolver {
	r, _ := ctx.Value(remoteNodeResolverKey{}).(RemoteNodeResolver)
	return r
}
