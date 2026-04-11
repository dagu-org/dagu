// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"

	"github.com/dagucloud/dagu/internal/agentoauth"
)

// Context keys for agent stores.
// These allow agent stores to be injected into Go contexts without
// creating a backwards dependency from the execution context to the agent package.

type configStoreKey struct{}
type modelStoreKey struct{}
type memoryStoreKey struct{}
type soulStoreKey struct{}
type remoteContextResolverKey struct{}
type oauthManagerKey struct{}

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

// WithRemoteContextResolver injects a RemoteContextResolver into the context.
func WithRemoteContextResolver(ctx context.Context, r RemoteContextResolver) context.Context {
	return context.WithValue(ctx, remoteContextResolverKey{}, r)
}

// GetRemoteContextResolver retrieves a RemoteContextResolver from the context.
// Returns nil if no RemoteContextResolver is set.
func GetRemoteContextResolver(ctx context.Context) RemoteContextResolver {
	r, _ := ctx.Value(remoteContextResolverKey{}).(RemoteContextResolver)
	return r
}

// WithOAuthManager injects the OAuth manager into the context.
func WithOAuthManager(ctx context.Context, m *agentoauth.Manager) context.Context {
	return context.WithValue(ctx, oauthManagerKey{}, m)
}

// GetOAuthManager retrieves the OAuth manager from the context.
func GetOAuthManager(ctx context.Context) *agentoauth.Manager {
	m, _ := ctx.Value(oauthManagerKey{}).(*agentoauth.Manager)
	return m
}
