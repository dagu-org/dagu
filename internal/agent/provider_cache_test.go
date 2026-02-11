package agent

import (
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderCache_GetOrCreate_CachedOnSecondCall(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()
	cfg := LLMConfig{Provider: "anthropic", Model: "claude-opus-4", APIKey: "key1"}
	mock := &mockLLMProvider{name: "test-provider"}

	// Pre-populate via Set so GetOrCreate does not call CreateLLMProvider.
	cache.Set(cfg, mock)

	p1, model1, err := cache.GetOrCreate(cfg)
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4", model1)

	p2, model2, err := cache.GetOrCreate(cfg)
	require.NoError(t, err)
	assert.Equal(t, "claude-opus-4", model2)

	// Both calls must return the exact same provider instance.
	assert.Same(t, p1, p2)
}

func TestProviderCache_Set(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()
	cfg := LLMConfig{Provider: "openai", Model: "gpt-4", APIKey: "sk-abc"}
	mock := &mockLLMProvider{name: "set-provider"}

	cache.Set(cfg, mock)

	p, model, err := cache.GetOrCreate(cfg)
	require.NoError(t, err)
	assert.Same(t, mock, p)
	assert.Equal(t, "gpt-4", model)
}

func TestProviderCache_Invalidate(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()
	cfg := LLMConfig{Provider: "anthropic", Model: "claude-opus-4", APIKey: "key"}

	mockA := &mockLLMProvider{name: "provider-a"}
	cache.Set(cfg, mockA)

	// Confirm it is cached.
	p, _, err := cache.GetOrCreate(cfg)
	require.NoError(t, err)
	assert.Same(t, mockA, p)

	// Invalidate the entry.
	cache.Invalidate(cfg)

	// After invalidation, Set a new provider to prove a fresh entry is used.
	mockB := &mockLLMProvider{name: "provider-b"}
	cache.Set(cfg, mockB)

	p2, _, err := cache.GetOrCreate(cfg)
	require.NoError(t, err)
	assert.Same(t, mockB, p2)
	assert.NotSame(t, mockA, p2)
}

func TestProviderCache_InvalidateAll(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()

	configs := []LLMConfig{
		{Provider: "anthropic", Model: "claude-opus-4", APIKey: "key1"},
		{Provider: "openai", Model: "gpt-4", APIKey: "key2"},
		{Provider: "gemini", Model: "gemini-pro", APIKey: "key3"},
	}

	for i, cfg := range configs {
		cache.Set(cfg, &mockLLMProvider{name: configs[i].Model})
	}

	cache.InvalidateAll()

	// After InvalidateAll, pre-populate one entry and verify the others are gone.
	cache.Set(configs[0], &mockLLMProvider{name: "fresh"})

	p, _, err := cache.GetOrCreate(configs[0])
	require.NoError(t, err)
	assert.Equal(t, "fresh", p.Name())

	// The internal cache should only have the one entry we just set.
	cache.mu.RLock()
	size := len(cache.cache)
	cache.mu.RUnlock()
	assert.Equal(t, 1, size)
}

func TestHashLLMConfig_Consistency(t *testing.T) {
	t.Parallel()

	cfg := LLMConfig{
		Provider: "anthropic",
		Model:    "claude-opus-4",
		APIKey:   "sk-abc123",
		BaseURL:  "https://api.anthropic.com",
	}

	hash1 := HashLLMConfig(cfg)
	hash2 := HashLLMConfig(cfg)

	assert.Equal(t, hash1, hash2, "same config must produce same hash")
	assert.Len(t, hash1, 64, "SHA-256 hex string should be 64 characters")
}

func TestHashLLMConfig_DifferentConfigs(t *testing.T) {
	t.Parallel()

	cfgA := LLMConfig{Provider: "anthropic", Model: "claude-opus-4", APIKey: "key"}
	cfgB := LLMConfig{Provider: "openai", Model: "gpt-4", APIKey: "key"}

	assert.NotEqual(t, HashLLMConfig(cfgA), HashLLMConfig(cfgB))
}

func TestHashLLMConfig_NoDelimiterInjection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    LLMConfig
		b    LLMConfig
	}{
		{
			name: "colon in provider vs model boundary",
			a:    LLMConfig{Provider: "abc", Model: "def", APIKey: "k", BaseURL: "u"},
			b:    LLMConfig{Provider: "ab", Model: "cdef", APIKey: "k", BaseURL: "u"},
		},
		{
			name: "pipe in values",
			a:    LLMConfig{Provider: "a|b", Model: "c", APIKey: "k", BaseURL: "u"},
			b:    LLMConfig{Provider: "a", Model: "b|c", APIKey: "k", BaseURL: "u"},
		},
		{
			name: "colon in api key",
			a:    LLMConfig{Provider: "p", Model: "m", APIKey: "key:1", BaseURL: "u"},
			b:    LLMConfig{Provider: "p", Model: "m", APIKey: "key", BaseURL: "1|u"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			hashA := HashLLMConfig(tc.a)
			hashB := HashLLMConfig(tc.b)
			assert.NotEqual(t, hashA, hashB,
				"configs with delimiter chars in values must produce different hashes")
		})
	}
}

func TestProviderCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()
	const goroutines = 50

	configs := make([]LLMConfig, goroutines)
	for i := range goroutines {
		configs[i] = LLMConfig{
			Provider: "anthropic",
			Model:    "model-" + strings.Repeat("x", i),
			APIKey:   "key",
		}
		cache.Set(configs[i], &mockLLMProvider{name: configs[i].Model})
	}

	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Concurrent GetOrCreate
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			_, _, _ = cache.GetOrCreate(configs[idx])
		}(i)
	}

	// Concurrent Set
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			cache.Set(configs[idx], &mockLLMProvider{name: "updated"})
		}(i)
	}

	// Concurrent Invalidate
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			cache.Invalidate(configs[idx])
		}(i)
	}

	wg.Wait()
	// If we reach here without a race detector failure, concurrency is safe.
}

func TestProviderCache_Eviction(t *testing.T) {
	t.Parallel()

	cache := NewProviderCache()
	cache.maxSize = 3

	cfgs := []LLMConfig{
		{Provider: "p", Model: "m1", APIKey: "k"},
		{Provider: "p", Model: "m2", APIKey: "k"},
		{Provider: "p", Model: "m3", APIKey: "k"},
	}

	// Fill to capacity.
	for _, cfg := range cfgs {
		cache.Set(cfg, &mockLLMProvider{name: cfg.Model})
	}

	cache.mu.RLock()
	assert.Equal(t, 3, len(cache.cache), "cache should have 3 entries at capacity")
	cache.mu.RUnlock()

	// Adding a 4th entry should trigger eviction (cache clears when full).
	extraCfg := LLMConfig{Provider: "p", Model: "m4", APIKey: "k"}
	cache.Set(extraCfg, &mockLLMProvider{name: "m4"})

	cache.mu.RLock()
	size := len(cache.cache)
	cache.mu.RUnlock()

	// After eviction the cache is cleared, then the new entry is added.
	assert.Equal(t, 1, size, "cache should have 1 entry after eviction clears everything")

	// The surviving entry should be the one just added.
	p, model, err := cache.GetOrCreate(extraCfg)
	require.NoError(t, err)
	assert.Equal(t, "m4", model)
	assert.Equal(t, "m4", p.Name())
}
