package fileagentconfig

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helpers

func setupTestStore(t *testing.T, opts ...Option) (*Store, string) {
	t.Helper()
	dataDir := t.TempDir()
	store, err := New(dataDir, opts...)
	require.NoError(t, err)
	return store, dataDir
}

func writeConfig(t *testing.T, dataDir string, cfg *agent.Config) {
	t.Helper()
	configPath := filepath.Join(dataDir, agentDirName, configFileName)
	data, err := json.Marshal(cfg)
	require.NoError(t, err)
	err = os.WriteFile(configPath, data, 0600)
	require.NoError(t, err)
}

func newTestConfig(enabled bool, provider, model string) *agent.Config {
	return &agent.Config{
		Enabled: enabled,
		LLM:     agent.LLMConfig{Provider: provider, Model: model},
	}
}

// mockProvider implements llm.Provider for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) Chat(_ context.Context, _ *llm.ChatRequest) (*llm.ChatResponse, error) {
	return nil, nil
}

func (m *mockProvider) ChatStream(_ context.Context, _ *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	return nil, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

var registerMockProvidersOnce sync.Once

func registerMockProviders() {
	registerMockProvidersOnce.Do(func() {
		providers := []llm.ProviderType{
			llm.ProviderAnthropic,
			llm.ProviderOpenAI,
			llm.ProviderGemini,
			llm.ProviderLocal,
		}
		for _, p := range providers {
			providerType := p
			llm.RegisterProvider(providerType, func(_ llm.Config) (llm.Provider, error) {
				return &mockProvider{name: string(providerType)}, nil
			})
		}
	})
}

func TestNew(t *testing.T) {
	t.Run("empty dataDir returns error", func(t *testing.T) {
		store, err := New("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot be empty")
		assert.Nil(t, store)
	})

	t.Run("valid dataDir creates store", func(t *testing.T) {
		dataDir := t.TempDir()
		store, err := New(dataDir)
		require.NoError(t, err)
		assert.NotNil(t, store)

		agentDir := filepath.Join(dataDir, agentDirName)
		info, err := os.Stat(agentDir)
		require.NoError(t, err)
		assert.True(t, info.IsDir())
	})

	t.Run("with config cache option", func(t *testing.T) {
		dataDir := t.TempDir()
		cache := fileutil.NewCache[*agent.Config]("test", 10, time.Hour)
		store, err := New(dataDir, WithConfigCache(cache))
		require.NoError(t, err)
		assert.NotNil(t, store)
	})
}

func TestNew_DirectoryCreationFailure(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where directory should be, blocking directory creation
	blockingFile := filepath.Join(tmpDir, agentDirName)
	require.NoError(t, os.WriteFile(blockingFile, []byte("block"), 0600))

	store, err := New(tmpDir)
	require.Error(t, err)
	assert.Nil(t, store)
	assert.Contains(t, err.Error(), "failed to create directory")
}

func TestWithConfigCache(t *testing.T) {
	cache := fileutil.NewCache[*agent.Config]("test-cache", 100, time.Hour)
	store := &Store{}
	WithConfigCache(cache)(store)
	assert.Equal(t, cache, store.configCache)
}

func TestStore_Load(t *testing.T) {
	ctx := context.Background()

	t.Run("file not exists returns default config", func(t *testing.T) {
		store, _ := setupTestStore(t)
		cfg, err := store.Load(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
	})

	t.Run("file exists with enabled true", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, "anthropic", "claude-sonnet-4-5"))

		cfg, err := store.Load(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
	})

	t.Run("file exists with enabled false", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(false, "openai", "gpt-4"))

		cfg, err := store.Load(ctx)
		require.NoError(t, err)
		assert.False(t, cfg.Enabled)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		configPath := filepath.Join(dataDir, agentDirName, configFileName)
		require.NoError(t, os.WriteFile(configPath, []byte("{invalid json}"), 0600))

		_, err := store.Load(ctx)
		require.Error(t, err)
	})
}

func TestStore_Load_WithCache(t *testing.T) {
	cache := fileutil.NewCache[*agent.Config]("test", 10, time.Hour)
	store, dataDir := setupTestStore(t, WithConfigCache(cache))
	ctx := context.Background()

	writeConfig(t, dataDir, newTestConfig(true, "anthropic", "test-model"))

	// First load reads from file
	cfg1, err := store.Load(ctx)
	require.NoError(t, err)
	assert.True(t, cfg1.Enabled)
	assert.Equal(t, "test-model", cfg1.LLM.Model)

	// Second load uses cache
	cfg2, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, cfg1.Enabled, cfg2.Enabled)
	assert.Equal(t, cfg1.LLM.Model, cfg2.LLM.Model)
}

func TestStore_Save(t *testing.T) {
	ctx := context.Background()

	t.Run("nil config returns error", func(t *testing.T) {
		store, _ := setupTestStore(t)
		err := store.Save(ctx, nil)
		require.Error(t, err)
	})

	t.Run("valid config saves successfully", func(t *testing.T) {
		store, _ := setupTestStore(t)
		cfg := &agent.Config{
			Enabled: true,
			LLM: agent.LLMConfig{
				Provider: "anthropic",
				Model:    "claude-sonnet-4-5",
				APIKey:   "test-key",
			},
		}

		require.NoError(t, store.Save(ctx, cfg))

		loaded, err := store.Load(ctx)
		require.NoError(t, err)
		assert.Equal(t, cfg.Enabled, loaded.Enabled)
		assert.Equal(t, cfg.LLM.Provider, loaded.LLM.Provider)
		assert.Equal(t, cfg.LLM.Model, loaded.LLM.Model)
		assert.Equal(t, cfg.LLM.APIKey, loaded.LLM.APIKey)
	})

	t.Run("config with all fields", func(t *testing.T) {
		store, _ := setupTestStore(t)
		cfg := &agent.Config{
			Enabled: false,
			LLM: agent.LLMConfig{
				Provider: "openai",
				Model:    "gpt-4",
				APIKey:   "sk-test",
				BaseURL:  "https://custom.api.com",
			},
		}

		require.NoError(t, store.Save(ctx, cfg))

		loaded, err := store.Load(ctx)
		require.NoError(t, err)
		assert.Equal(t, cfg.Enabled, loaded.Enabled)
		assert.Equal(t, cfg.LLM.Provider, loaded.LLM.Provider)
		assert.Equal(t, cfg.LLM.Model, loaded.LLM.Model)
		assert.Equal(t, cfg.LLM.APIKey, loaded.LLM.APIKey)
		assert.Equal(t, cfg.LLM.BaseURL, loaded.LLM.BaseURL)
	})
}

func TestStore_IsEnabled(t *testing.T) {
	ctx := context.Background()

	t.Run("default is enabled", func(t *testing.T) {
		store, _ := setupTestStore(t)
		assert.True(t, store.IsEnabled(ctx))
	})

	t.Run("enabled true in config", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, "anthropic", "model"))
		assert.True(t, store.IsEnabled(ctx))
	})

	t.Run("enabled false in config", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(false, "anthropic", "model"))
		assert.False(t, store.IsEnabled(ctx))
	})

	t.Run("invalid config returns false", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		configPath := filepath.Join(dataDir, agentDirName, configFileName)
		require.NoError(t, os.WriteFile(configPath, []byte("not json"), 0600))
		assert.False(t, store.IsEnabled(ctx))
	})
}

func TestStore_GetProvider(t *testing.T) {
	registerMockProviders()
	ctx := context.Background()

	t.Run("agent enabled returns provider", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, "anthropic", "claude-sonnet-4-5"))

		provider, model, err := store.GetProvider(ctx)
		require.NoError(t, err)
		assert.NotNil(t, provider)
		assert.NotEmpty(t, model)
	})

	t.Run("agent disabled returns error", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(false, "anthropic", "claude-sonnet-4-5"))

		provider, model, err := store.GetProvider(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "agent is disabled")
		assert.Nil(t, provider)
		assert.Empty(t, model)
	})

	t.Run("invalid provider returns error", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, "invalid-provider", "test"))

		provider, model, err := store.GetProvider(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid")
		assert.Nil(t, provider)
		assert.Empty(t, model)
	})
}

func TestStore_Exists(t *testing.T) {
	t.Run("file exists returns true", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, "anthropic", "model"))
		assert.True(t, store.Exists())
	})

	t.Run("file not exists returns false", func(t *testing.T) {
		store, _ := setupTestStore(t)
		assert.False(t, store.Exists())
	})
}

func TestApplyEnvOverrides(t *testing.T) {
	baseConfig := func() *agent.Config {
		return newTestConfig(true, "anthropic", "claude-sonnet-4-5")
	}

	t.Run("override enabled to false", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "false")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.False(t, cfg.Enabled)
	})

	t.Run("override enabled to true", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "true")
		cfg := newTestConfig(false, "anthropic", "claude-sonnet-4-5")
		applyEnvOverrides(cfg)
		assert.True(t, cfg.Enabled)
	})

	t.Run("override provider", func(t *testing.T) {
		t.Setenv(envAgentLLMProvider, "openai")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.Equal(t, "openai", cfg.LLM.Provider)
	})

	t.Run("override model", func(t *testing.T) {
		t.Setenv(envAgentLLMModel, "gpt-4o")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.Equal(t, "gpt-4o", cfg.LLM.Model)
	})

	t.Run("override api key", func(t *testing.T) {
		t.Setenv(envAgentLLMAPIKey, "env-api-key")
		cfg := baseConfig()
		cfg.LLM.APIKey = "file-key"
		applyEnvOverrides(cfg)
		assert.Equal(t, "env-api-key", cfg.LLM.APIKey)
	})

	t.Run("override base url", func(t *testing.T) {
		t.Setenv(envAgentLLMBaseURL, "https://custom.api.com")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.Equal(t, "https://custom.api.com", cfg.LLM.BaseURL)
	})

	t.Run("override all fields", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "false")
		t.Setenv(envAgentLLMProvider, "openai")
		t.Setenv(envAgentLLMModel, "gpt-4o")
		t.Setenv(envAgentLLMAPIKey, "env-key")
		t.Setenv(envAgentLLMBaseURL, "https://api.openai.com/v1")

		cfg := baseConfig()
		applyEnvOverrides(cfg)

		assert.False(t, cfg.Enabled)
		assert.Equal(t, "openai", cfg.LLM.Provider)
		assert.Equal(t, "gpt-4o", cfg.LLM.Model)
		assert.Equal(t, "env-key", cfg.LLM.APIKey)
		assert.Equal(t, "https://api.openai.com/v1", cfg.LLM.BaseURL)
	})

	t.Run("invalid enabled value is ignored", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "invalid")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.True(t, cfg.Enabled)
	})
}

func TestHashLLMConfig(t *testing.T) {
	baseCfg := agent.LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5", APIKey: "key1"}

	t.Run("same config produces same hash", func(t *testing.T) {
		cfg1 := baseCfg
		cfg2 := baseCfg
		assert.Equal(t, hashLLMConfig(cfg1), hashLLMConfig(cfg2))
	})

	t.Run("hash is deterministic", func(t *testing.T) {
		hash1 := hashLLMConfig(baseCfg)
		hash2 := hashLLMConfig(baseCfg)
		assert.Equal(t, hash1, hash2)
	})

	t.Run("different provider produces different hash", func(t *testing.T) {
		cfg1 := baseCfg
		cfg2 := baseCfg
		cfg2.Provider = "openai"
		assert.NotEqual(t, hashLLMConfig(cfg1), hashLLMConfig(cfg2))
	})

	t.Run("different model produces different hash", func(t *testing.T) {
		cfg1 := baseCfg
		cfg2 := baseCfg
		cfg2.Model = "claude-opus-4"
		assert.NotEqual(t, hashLLMConfig(cfg1), hashLLMConfig(cfg2))
	})

	t.Run("different api key produces different hash", func(t *testing.T) {
		cfg1 := baseCfg
		cfg2 := baseCfg
		cfg2.APIKey = "key2"
		assert.NotEqual(t, hashLLMConfig(cfg1), hashLLMConfig(cfg2))
	})

	t.Run("different base url produces different hash", func(t *testing.T) {
		cfg1 := baseCfg
		cfg2 := baseCfg
		cfg1.BaseURL = "url1"
		cfg2.BaseURL = "url2"
		assert.NotEqual(t, hashLLMConfig(cfg1), hashLLMConfig(cfg2))
	})
}

func TestCreateLLMProvider(t *testing.T) {
	registerMockProviders()

	validProviders := []struct {
		provider string
		model    string
	}{
		{"anthropic", "claude-sonnet-4-5"},
		{"openai", "gpt-4"},
		{"gemini", "gemini-pro"},
		{"local", "llama2"},
	}

	for _, tc := range validProviders {
		t.Run("valid "+tc.provider+" provider", func(t *testing.T) {
			cfg := agent.LLMConfig{Provider: tc.provider, Model: tc.model}
			provider, err := createLLMProvider(cfg)
			require.NoError(t, err)
			assert.NotNil(t, provider)
		})
	}

	t.Run("invalid provider type", func(t *testing.T) {
		cfg := agent.LLMConfig{Provider: "nonexistent", Model: "test"}
		provider, err := createLLMProvider(cfg)
		require.Error(t, err)
		assert.Nil(t, provider)
	})

	t.Run("with custom api key", func(t *testing.T) {
		cfg := agent.LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5", APIKey: "custom-key"}
		provider, err := createLLMProvider(cfg)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})

	t.Run("with custom base url", func(t *testing.T) {
		cfg := agent.LLMConfig{Provider: "openai", Model: "gpt-4", BaseURL: "https://custom.api.com"}
		provider, err := createLLMProvider(cfg)
		require.NoError(t, err)
		assert.NotNil(t, provider)
	})
}

func TestProviderCache(t *testing.T) {
	registerMockProviders()

	t.Run("cache hit returns same provider", func(t *testing.T) {
		cache := newProviderCache()
		cfg := agent.LLMConfig{Provider: "anthropic", Model: "claude-sonnet-4-5", APIKey: "key1"}

		p1, m1, err := cache.get(cfg)
		require.NoError(t, err)
		assert.NotNil(t, p1)
		assert.Equal(t, "claude-sonnet-4-5", m1)

		p2, m2, err := cache.get(cfg)
		require.NoError(t, err)
		assert.Equal(t, p1, p2, "same provider should be returned from cache")
		assert.Equal(t, m1, m2)
	})

	t.Run("cache miss creates new provider", func(t *testing.T) {
		cache := newProviderCache()
		cfg1 := agent.LLMConfig{Provider: "anthropic", Model: "model1", APIKey: "key1"}
		cfg2 := agent.LLMConfig{Provider: "anthropic", Model: "model2", APIKey: "key1"}

		_, _, err := cache.get(cfg1)
		require.NoError(t, err)

		p2, m2, err := cache.get(cfg2)
		require.NoError(t, err)
		assert.NotNil(t, p2)
		assert.Equal(t, "model2", m2)
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := agent.DefaultConfig()
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, agent.DefaultProvider, cfg.LLM.Provider)
	assert.Equal(t, agent.DefaultModel, cfg.LLM.Model)
	assert.Empty(t, cfg.LLM.APIKey)
	assert.Empty(t, cfg.LLM.BaseURL)
}

func TestStore_ConfigPath(t *testing.T) {
	store, dataDir := setupTestStore(t)
	expectedPath := filepath.Join(dataDir, agentDirName, configFileName)
	assert.Equal(t, expectedPath, store.configPath())
}

func TestStore_GetProvider_LoadError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	configPath := filepath.Join(dataDir, agentDirName, configFileName)
	require.NoError(t, os.WriteFile(configPath, []byte("{invalid}"), 0600))

	provider, model, err := store.GetProvider(context.Background())
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Empty(t, model)
}

func TestStore_Save_WriteError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	agentDir := filepath.Join(dataDir, agentDirName)

	// Make directory read-only to cause write failure
	require.NoError(t, os.Chmod(agentDir, 0500))
	defer func() { _ = os.Chmod(agentDir, 0750) }()

	cfg := newTestConfig(true, "anthropic", "model")
	err := store.Save(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write")
}

func TestProviderCache_Error(t *testing.T) {
	cache := newProviderCache()
	cfg := agent.LLMConfig{Provider: "invalid-provider", Model: "test"}

	provider, model, err := cache.get(cfg)
	require.Error(t, err)
	assert.Nil(t, provider)
	assert.Empty(t, model)
}

func TestStore_Load_ReadPermissionError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	writeConfig(t, dataDir, newTestConfig(true, "anthropic", "model"))

	configPath := filepath.Join(dataDir, agentDirName, configFileName)
	require.NoError(t, os.Chmod(configPath, 0000))
	defer func() { _ = os.Chmod(configPath, 0600) }()

	cfg, err := store.Load(context.Background())
	require.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestStore_Save_RenameError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	configPath := filepath.Join(dataDir, agentDirName, configFileName)

	// Create directory where file should be to cause rename failure
	require.NoError(t, os.Mkdir(configPath, 0750))

	cfg := newTestConfig(true, "anthropic", "model")
	err := store.Save(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to rename")
}
