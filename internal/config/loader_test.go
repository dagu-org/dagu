package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLoader_EnvironmentVariableBindings tests that all environment variables
// defined in bindEnvironmentVariables() are correctly bound and applied to the config
func TestConfigLoader_EnvironmentVariableBindings(t *testing.T) {
	// Reset viper to ensure clean state
	viper.Reset()
	defer viper.Reset()

	// Create a minimal config file
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("# minimal config"), 0600)
	require.NoError(t, err)

	// Define all environment variables that should be bound according to bindEnvironmentVariables()
	testEnvs := map[string]string{
		// Server configurations
		"DAGU_LOG_FORMAT":   "json",
		"DAGU_BASE_PATH":    "/test/base",
		"DAGU_API_BASE_URL": "/test/api",
		"DAGU_TZ":           "Europe/Berlin",
		"DAGU_HOST":         "test.example.com",
		"DAGU_PORT":         "9876",
		"DAGU_DEBUG":        "true",
		"DAGU_HEADLESS":     "true",

		// Global configurations
		"DAGU_WORK_DIR":      "/test/work",
		"DAGU_DEFAULT_SHELL": "/bin/zsh",

		// UI configurations (new keys)
		"DAGU_UI_MAX_DASHBOARD_PAGE_LIMIT": "250",
		"DAGU_UI_LOG_ENCODING_CHARSET":     "iso-8859-1",
		"DAGU_UI_NAVBAR_COLOR":             "#123456",
		"DAGU_UI_NAVBAR_TITLE":             "Test Dagu",

		// Authentication configurations (new keys)
		"DAGU_AUTH_BASIC_USERNAME":   "testuser",
		"DAGU_AUTH_BASIC_PASSWORD":   "testpass",
		"DAGU_AUTH_TOKEN":            "test-token-123",
		"DAGU_AUTH_NODE_SIGNING_KEY": "test-signing-key-abc123",

		// TLS configurations
		"DAGU_CERT_FILE": "/test/cert.pem",
		"DAGU_KEY_FILE":  "/test/key.pem",

		// File paths
		"DAGU_DAGS_DIR":          "/test/dags",
		"DAGU_EXECUTABLE":        "/test/bin/dagu",
		"DAGU_LOG_DIR":           "/test/logs",
		"DAGU_DATA_DIR":          "/test/data",
		"DAGU_SUSPEND_FLAGS_DIR": "/test/suspend",
		"DAGU_ADMIN_LOG_DIR":     "/test/admin",
		"DAGU_BASE_CONFIG":       "/test/base.yaml",
		"DAGU_DAG_RUNS_DIR":      "/test/runs",
		"DAGU_PROC_DIR":          "/test/proc",
		"DAGU_QUEUE_DIR":         "/test/queue",

		// UI customization
		"DAGU_LATEST_STATUS_TODAY": "true",

		// Queue configuration
		"DAGU_QUEUE_ENABLED": "false",
	}

	// Save and clear existing environment variables
	savedEnvs := make(map[string]string)
	for key := range testEnvs {
		savedEnvs[key] = os.Getenv(key)
		os.Unsetenv(key)
	}
	defer func() {
		// Restore original environment
		for key, val := range savedEnvs {
			if val != "" {
				os.Setenv(key, val)
			} else {
				os.Unsetenv(key)
			}
		}
	}()

	// Set test environment variables
	for key, val := range testEnvs {
		os.Setenv(key, val)
	}

	// Load configuration
	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify all environment variables were correctly bound and applied

	// Server configurations
	assert.Equal(t, "json", cfg.Global.LogFormat)
	assert.Equal(t, "/test/base", cfg.Server.BasePath)
	assert.Equal(t, "/test/api", cfg.Server.APIBasePath)
	assert.Equal(t, "Europe/Berlin", cfg.Global.TZ)
	assert.Equal(t, "test.example.com", cfg.Server.Host)
	assert.Equal(t, 9876, cfg.Server.Port)
	assert.True(t, cfg.Global.Debug)
	assert.True(t, cfg.Server.Headless)

	// Global configurations
	assert.Equal(t, "/test/work", cfg.Global.WorkDir)
	assert.Equal(t, "/bin/zsh", cfg.Global.DefaultShell)

	// UI configurations
	assert.Equal(t, 250, cfg.UI.MaxDashboardPageLimit)
	assert.Equal(t, "iso-8859-1", cfg.UI.LogEncodingCharset)
	assert.Equal(t, "#123456", cfg.UI.NavbarColor)
	assert.Equal(t, "Test Dagu", cfg.UI.NavbarTitle)

	// Authentication configurations
	assert.Equal(t, "testuser", cfg.Server.Auth.Basic.Username)
	assert.Equal(t, "testpass", cfg.Server.Auth.Basic.Password)
	assert.Equal(t, "test-token-123", cfg.Server.Auth.Token.Value)
	assert.Equal(t, "test-signing-key-abc123", cfg.Server.Auth.NodeSigningKey)
	assert.True(t, cfg.Server.Auth.Basic.Enabled())
	assert.True(t, cfg.Server.Auth.Token.Enabled())

	// TLS configurations
	require.NotNil(t, cfg.Server.TLS)
	assert.Equal(t, "/test/cert.pem", cfg.Server.TLS.CertFile)
	assert.Equal(t, "/test/key.pem", cfg.Server.TLS.KeyFile)

	// File paths
	assert.Equal(t, "/test/dags", cfg.Paths.DAGsDir)
	assert.Equal(t, "/test/bin/dagu", cfg.Paths.Executable)
	assert.Equal(t, "/test/logs", cfg.Paths.LogDir)
	assert.Equal(t, "/test/data", cfg.Paths.DataDir)
	assert.Equal(t, "/test/suspend", cfg.Paths.SuspendFlagsDir)
	assert.Equal(t, "/test/admin", cfg.Paths.AdminLogsDir)
	assert.Equal(t, "/test/base.yaml", cfg.Paths.BaseConfig)
	assert.Equal(t, "/test/runs", cfg.Paths.DAGRunsDir)
	assert.Equal(t, "/test/proc", cfg.Paths.ProcDir)
	assert.Equal(t, "/test/queue", cfg.Paths.QueueDir)

	// UI customization
	assert.True(t, cfg.Server.LatestStatusToday)

	// Queue configuration
	assert.False(t, cfg.Queues.Enabled)
}

// TestConfigLoader_NodeSigningKey tests the NodeSigningKey configuration
func TestConfigLoader_NodeSigningKey(t *testing.T) {
	t.Run("LoadFromYAML", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with NodeSigningKey
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  nodeSigningKey: "yaml-signing-key-123"
  basic:
    username: "admin"
    password: "pass"
  token:
    value: "api-token"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify NodeSigningKey is loaded from YAML
		assert.Equal(t, "yaml-signing-key-123", cfg.Server.Auth.NodeSigningKey)
		assert.Equal(t, "admin", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "api-token", cfg.Server.Auth.Token.Value)
	})

	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with NodeSigningKey
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  nodeSigningKey: "yaml-signing-key"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variable
		os.Setenv("DAGU_AUTH_NODE_SIGNING_KEY", "env-signing-key-override")
		defer os.Unsetenv("DAGU_AUTH_NODE_SIGNING_KEY")

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify environment variable overrides YAML
		assert.Equal(t, "env-signing-key-override", cfg.Server.Auth.NodeSigningKey)
	})

	t.Run("EmptyNodeSigningKey", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file without NodeSigningKey
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  basic:
    username: "user"
    password: "pass"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify NodeSigningKey is empty when not provided
		assert.Equal(t, "", cfg.Server.Auth.NodeSigningKey)
		assert.Equal(t, "user", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
	})

	t.Run("NestedAuthConfig", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config with all auth fields
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
  nodeSigningKey: "master-signing-key"
  basic:
    username: "testuser"
    password: "testpass"
  token:
    value: "test-token"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify all auth fields are loaded correctly
		assert.Equal(t, "master-signing-key", cfg.Server.Auth.NodeSigningKey)
		assert.Equal(t, "testuser", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "testpass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "test-token", cfg.Server.Auth.Token.Value)
	})
}
