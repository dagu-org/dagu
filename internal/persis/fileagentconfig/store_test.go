package fileagentconfig

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
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

func newTestConfig(enabled bool, defaultModelID string) *agent.Config {
	return &agent.Config{
		Enabled:        enabled,
		DefaultModelID: defaultModelID,
	}
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
		writeConfig(t, dataDir, newTestConfig(true, "claude-sonnet-4-5"))

		cfg, err := store.Load(ctx)
		require.NoError(t, err)
		assert.True(t, cfg.Enabled)
		assert.Equal(t, "claude-sonnet-4-5", cfg.DefaultModelID)
	})

	t.Run("file exists with enabled false", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(false, "gpt-4"))

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

	writeConfig(t, dataDir, newTestConfig(true, "test-model"))

	// First load reads from file
	cfg1, err := store.Load(ctx)
	require.NoError(t, err)
	assert.True(t, cfg1.Enabled)
	assert.Equal(t, "test-model", cfg1.DefaultModelID)

	// Second load uses cache
	cfg2, err := store.Load(ctx)
	require.NoError(t, err)
	assert.Equal(t, cfg1.Enabled, cfg2.Enabled)
	assert.Equal(t, cfg1.DefaultModelID, cfg2.DefaultModelID)
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
			Enabled:        true,
			DefaultModelID: "claude-sonnet-4-5",
		}

		require.NoError(t, store.Save(ctx, cfg))

		loaded, err := store.Load(ctx)
		require.NoError(t, err)
		assert.Equal(t, cfg.Enabled, loaded.Enabled)
		assert.Equal(t, cfg.DefaultModelID, loaded.DefaultModelID)
	})

	t.Run("config with all fields", func(t *testing.T) {
		store, _ := setupTestStore(t)
		cfg := &agent.Config{
			Enabled:        false,
			DefaultModelID: "gpt-4-1",
		}

		require.NoError(t, store.Save(ctx, cfg))

		loaded, err := store.Load(ctx)
		require.NoError(t, err)
		assert.Equal(t, cfg.Enabled, loaded.Enabled)
		assert.Equal(t, cfg.DefaultModelID, loaded.DefaultModelID)
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
		writeConfig(t, dataDir, newTestConfig(true, ""))
		assert.True(t, store.IsEnabled(ctx))
	})

	t.Run("enabled false in config", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(false, ""))
		assert.False(t, store.IsEnabled(ctx))
	})

	t.Run("invalid config returns false", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		configPath := filepath.Join(dataDir, agentDirName, configFileName)
		require.NoError(t, os.WriteFile(configPath, []byte("not json"), 0600))
		assert.False(t, store.IsEnabled(ctx))
	})
}

func TestStore_Exists(t *testing.T) {
	t.Run("file exists returns true", func(t *testing.T) {
		store, dataDir := setupTestStore(t)
		writeConfig(t, dataDir, newTestConfig(true, ""))
		assert.True(t, store.Exists())
	})

	t.Run("file not exists returns false", func(t *testing.T) {
		store, _ := setupTestStore(t)
		assert.False(t, store.Exists())
	})
}

func TestApplyEnvOverrides(t *testing.T) {
	baseConfig := func() *agent.Config {
		return newTestConfig(true, "claude-sonnet-4-5")
	}

	t.Run("override enabled to false", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "false")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.False(t, cfg.Enabled)
	})

	t.Run("override enabled to true", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "true")
		cfg := newTestConfig(false, "claude-sonnet-4-5")
		applyEnvOverrides(cfg)
		assert.True(t, cfg.Enabled)
	})

	t.Run("invalid enabled value is ignored", func(t *testing.T) {
		t.Setenv(envAgentEnabled, "invalid")
		cfg := baseConfig()
		applyEnvOverrides(cfg)
		assert.True(t, cfg.Enabled)
	})
}

func TestDefaultConfig(t *testing.T) {
	cfg := agent.DefaultConfig()
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Enabled)
	assert.Empty(t, cfg.DefaultModelID)
}

func TestStore_ConfigPath(t *testing.T) {
	store, dataDir := setupTestStore(t)
	expectedPath := filepath.Join(dataDir, agentDirName, configFileName)
	assert.Equal(t, expectedPath, store.configPath())
}

func TestStore_Save_WriteError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	agentDir := filepath.Join(dataDir, agentDirName)

	// Make directory read-only to cause write failure
	require.NoError(t, os.Chmod(agentDir, 0500))
	defer func() { _ = os.Chmod(agentDir, 0750) }()

	cfg := newTestConfig(true, "")
	err := store.Save(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create temp file")
}

func TestStore_Load_ReadPermissionError(t *testing.T) {
	store, dataDir := setupTestStore(t)
	writeConfig(t, dataDir, newTestConfig(true, ""))

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

	cfg := newTestConfig(true, "")
	err := store.Save(context.Background(), cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to rename")
}
