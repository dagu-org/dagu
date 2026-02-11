package agent

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"

	"github.com/dagu-org/dagu/internal/llm"
)

// ProviderCache caches LLM providers keyed by config hash.
// Safe for concurrent use.
type ProviderCache struct {
	mu    sync.RWMutex
	cache map[string]cachedProvider
}

type cachedProvider struct {
	provider llm.Provider
	model    string
}

// NewProviderCache creates a new empty provider cache.
func NewProviderCache() *ProviderCache {
	return &ProviderCache{
		cache: make(map[string]cachedProvider),
	}
}

// Set stores a provider in the cache for the given config.
// Useful for testing to inject mock providers.
func (c *ProviderCache) Set(cfg LLMConfig, provider llm.Provider) {
	hash := HashLLMConfig(cfg)
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[hash] = cachedProvider{
		provider: provider,
		model:    cfg.Model,
	}
}

// GetOrCreate returns a cached provider for the given config, or creates one.
func (c *ProviderCache) GetOrCreate(cfg LLMConfig) (llm.Provider, string, error) {
	hash := HashLLMConfig(cfg)

	c.mu.RLock()
	if entry, ok := c.cache[hash]; ok {
		c.mu.RUnlock()
		return entry.provider, entry.model, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after write lock
	if entry, ok := c.cache[hash]; ok {
		return entry.provider, entry.model, nil
	}

	provider, err := CreateLLMProvider(cfg)
	if err != nil {
		return nil, "", err
	}

	c.cache[hash] = cachedProvider{
		provider: provider,
		model:    cfg.Model,
	}
	return provider, cfg.Model, nil
}

// HashLLMConfig creates a hash of the LLM config for cache invalidation.
func HashLLMConfig(cfg LLMConfig) string {
	data := fmt.Sprintf("%s:%s:%s:%s", cfg.Provider, cfg.Model, cfg.APIKey, cfg.BaseURL)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:8])
}

// CreateLLMProvider creates an LLM provider from config.
func CreateLLMProvider(agentCfg LLMConfig) (llm.Provider, error) {
	providerType, err := llm.ParseProviderType(agentCfg.Provider)
	if err != nil {
		return nil, fmt.Errorf("invalid LLM provider: %w", err)
	}

	cfg := llm.DefaultConfig()
	cfg.APIKey = cmp.Or(agentCfg.APIKey, cfg.APIKey)
	cfg.BaseURL = cmp.Or(agentCfg.BaseURL, cfg.BaseURL)

	return llm.NewProvider(providerType, cfg)
}
