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
		"DAGU_AUTH_BASIC_USERNAME": "testuser",
		"DAGU_AUTH_BASIC_PASSWORD": "testpass",
		"DAGU_AUTH_TOKEN":          "test-token-123",

		// TLS configurations
		"DAGU_CERT_FILE": "/test/cert.pem",
		"DAGU_KEY_FILE":  "/test/key.pem",

		// File paths
		"DAGU_DAGS_DIR":             "/test/dags",
		"DAGU_EXECUTABLE":           "/test/bin/dagu",
		"DAGU_LOG_DIR":              "/test/logs",
		"DAGU_DATA_DIR":             "/test/data",
		"DAGU_SUSPEND_FLAGS_DIR":    "/test/suspend",
		"DAGU_ADMIN_LOG_DIR":        "/test/admin",
		"DAGU_BASE_CONFIG":          "/test/base.yaml",
		"DAGU_DAG_RUNS_DIR":         "/test/runs",
		"DAGU_PROC_DIR":             "/test/proc",
		"DAGU_QUEUE_DIR":            "/test/queue",
		"DAGU_SERVICE_REGISTRY_DIR": "/test/service-registry",

		// UI customization
		"DAGU_LATEST_STATUS_TODAY": "true",

		// Queue configuration
		"DAGU_QUEUE_ENABLED": "false",

		// Worker configuration - env vars still bound but to nested structure
		"DAGU_WORKER_ID":              "test-worker-123",
		"DAGU_WORKER_MAX_ACTIVE_RUNS": "200",

		// Scheduler configuration
		"DAGU_SCHEDULER_PORT": "9999",
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
	assert.True(t, cfg.Server.Auth.Basic.Enabled())
	assert.True(t, cfg.Server.Auth.Token.Enabled())

	// Coordinator configurations are loaded from config file only

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
	assert.Equal(t, "/test/service-registry", cfg.Paths.ServiceRegistryDir)

	// UI customization
	assert.True(t, cfg.Server.LatestStatusToday)

	// Queue configuration
	assert.False(t, cfg.Queues.Enabled)

	// Worker configuration
	assert.Equal(t, "test-worker-123", cfg.Worker.ID)
	assert.Equal(t, 200, cfg.Worker.MaxActiveRuns)

	// Scheduler configuration
	assert.Equal(t, 9999, cfg.Scheduler.Port)
}

func TestConfigLoader_CoordinatorSigningKey(t *testing.T) {
	t.Run("LoadFromYAML", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with auth configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
auth:
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

		// Verify auth configuration is loaded from YAML
		assert.Equal(t, "admin", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "api-token", cfg.Server.Auth.Token.Value)
	})

	t.Run("EmptyCoordinatorSigningKey", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file with basic auth only
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

		// Verify auth configuration
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
		assert.Equal(t, "testuser", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "testpass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "test-token", cfg.Server.Auth.Token.Value)
	})
}

func TestConfigLoader_WorkerConfiguration(t *testing.T) {
	t.Run("LoadFromYAML", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with Worker configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  id: "yaml-worker-01"
  maxActiveRuns: 50
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify Worker configuration is loaded from YAML
		assert.Equal(t, "yaml-worker-01", cfg.Worker.ID)
		assert.Equal(t, 50, cfg.Worker.MaxActiveRuns)
	})

	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with Worker configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  id: "yaml-worker"
  maxActiveRuns: 10
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variables
		envs := map[string]string{
			"DAGU_WORKER_ID":              "env-worker-override",
			"DAGU_WORKER_MAX_ACTIVE_RUNS": "300",
		}

		// Save and clear existing environment variables
		savedEnvs := make(map[string]string)
		for key := range envs {
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
		for key, val := range envs {
			os.Setenv(key, val)
		}

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify environment variables override YAML
		assert.Equal(t, "env-worker-override", cfg.Worker.ID)
		assert.Equal(t, 300, cfg.Worker.MaxActiveRuns)
	})

	t.Run("DefaultValues", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file without Worker configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
host: "localhost"
port: 8080
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify default Worker configuration values
		assert.Equal(t, "", cfg.Worker.ID) // No default ID
		assert.Equal(t, 100, cfg.Worker.MaxActiveRuns)
	})

	t.Run("SecureByDefault", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with only worker host/port
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  coordinatorHost: "secure.example.com"
  coordinatorPort: 443
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)
	})
}

func TestWorkerLabels(t *testing.T) {
	t.Run("LabelsFromString", func(t *testing.T) {
		// Create config with worker labels as string
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  labels: "gpu=true,memory=64G,region=us-east-1"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify labels are parsed correctly
		expected := map[string]string{
			"gpu":    "true",
			"memory": "64G",
			"region": "us-east-1",
		}
		assert.Equal(t, expected, cfg.Worker.Labels)
	})

	t.Run("LabelsFromMap", func(t *testing.T) {
		// Create config with worker labels as map
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  labels:
    gpu: "true"
    memory: "64G"
    region: "us-west-2"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify labels are loaded correctly
		expected := map[string]string{
			"gpu":    "true",
			"memory": "64G",
			"region": "us-west-2",
		}
		assert.Equal(t, expected, cfg.Worker.Labels)
	})

	t.Run("LabelsFromEnvironment", func(t *testing.T) {
		// Set environment variable
		os.Setenv("DAGU_WORKER_LABELS", "instance-type=m5.xlarge,cpu-arch=amd64")
		defer os.Unsetenv("DAGU_WORKER_LABELS")

		// Create minimal config
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		err := os.WriteFile(configFile, []byte("# minimal config"), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify labels from environment are parsed correctly
		expected := map[string]string{
			"instance-type": "m5.xlarge",
			"cpu-arch":      "amd64",
		}
		assert.Equal(t, expected, cfg.Worker.Labels)
	})

	t.Run("EmptyLabels", func(t *testing.T) {
		// Create config without worker labels
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
worker:
  maxActiveRuns: 50
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify labels are nil or empty
		assert.True(t, len(cfg.Worker.Labels) == 0)
	})
}

func TestDefaultDirectoryConfiguration(t *testing.T) {
	t.Run("DefaultDirectoriesUnderDataDir", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file with only data dir specified
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
paths:
  dataDir: "/custom/data"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify default directories are created under data dir
		assert.Equal(t, "/custom/data", cfg.Paths.DataDir)
		assert.Equal(t, "/custom/data/dag-runs", cfg.Paths.DAGRunsDir)
		assert.Equal(t, "/custom/data/proc", cfg.Paths.ProcDir)
		assert.Equal(t, "/custom/data/queue", cfg.Paths.QueueDir)
		assert.Equal(t, "/custom/data/service-registry", cfg.Paths.ServiceRegistryDir)
	})
}

func TestPeerConfiguration(t *testing.T) {
	t.Run("LoadFromYAML", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with Peer configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
peer:
  certFile: "/path/to/peer/cert.pem"
  keyFile: "/path/to/peer/key.pem"
  clientCaFile: "/path/to/peer/ca.pem"
  skipTlsVerify: true
  insecure: false
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify Peer configuration is loaded from YAML
		assert.Equal(t, "/path/to/peer/cert.pem", cfg.Global.Peer.CertFile)
		assert.Equal(t, "/path/to/peer/key.pem", cfg.Global.Peer.KeyFile)
		assert.Equal(t, "/path/to/peer/ca.pem", cfg.Global.Peer.ClientCaFile)
		assert.True(t, cfg.Global.Peer.SkipTLSVerify)
		assert.False(t, cfg.Global.Peer.Insecure)
	})

	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with Peer configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
peer:
  certFile: "/yaml/cert.pem"
  keyFile: "/yaml/key.pem"
  skipTlsVerify: false
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variables
		envs := map[string]string{
			"DAGU_PEER_CERT_FILE":       "/env/cert.pem",
			"DAGU_PEER_KEY_FILE":        "/env/key.pem",
			"DAGU_PEER_CLIENT_CA_FILE":  "/env/ca.pem",
			"DAGU_PEER_SKIP_TLS_VERIFY": "true",
			"DAGU_PEER_INSECURE":        "false",
		}

		// Save and clear existing environment variables
		savedEnvs := make(map[string]string)
		for key := range envs {
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
		for key, val := range envs {
			os.Setenv(key, val)
		}

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify environment variables override YAML
		assert.Equal(t, "/env/cert.pem", cfg.Global.Peer.CertFile)
		assert.Equal(t, "/env/key.pem", cfg.Global.Peer.KeyFile)
		assert.Equal(t, "/env/ca.pem", cfg.Global.Peer.ClientCaFile)
		assert.True(t, cfg.Global.Peer.SkipTLSVerify)
		assert.False(t, cfg.Global.Peer.Insecure) // overridden by env var
	})

	t.Run("DefaultValues", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file without Peer configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
host: "localhost"
port: 8080
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify default Peer configuration values
		assert.Equal(t, "", cfg.Global.Peer.CertFile)
		assert.Equal(t, "", cfg.Global.Peer.KeyFile)
		assert.Equal(t, "", cfg.Global.Peer.ClientCaFile)
		assert.False(t, cfg.Global.Peer.SkipTLSVerify)
		assert.True(t, cfg.Global.Peer.Insecure) // default is true
	})

	t.Run("PartialPeerConfig", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with partial Peer configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
peer:
  certFile: "/path/to/cert.pem"
  skipTlsVerify: true
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify partial Peer config is loaded correctly
		assert.Equal(t, "/path/to/cert.pem", cfg.Global.Peer.CertFile)
		assert.Equal(t, "", cfg.Global.Peer.KeyFile)
		assert.Equal(t, "", cfg.Global.Peer.ClientCaFile)
		assert.True(t, cfg.Global.Peer.SkipTLSVerify)
	})
}
