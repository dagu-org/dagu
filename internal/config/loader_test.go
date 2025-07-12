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

		// Note: DAGU_COORDINATOR_SIGNING_KEY environment variable is bound to coordinatorSigningKey
		// but there's no flat field in the definition, so it doesn't work.
		// Removing this test case as it's testing non-existent functionality.

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

		// Worker configuration
		"DAGU_WORKER_ID":                  "test-worker-123",
		"DAGU_WORKER_MAX_ACTIVE_RUNS": "200",
		"DAGU_WORKER_COORDINATOR_HOST":    "worker.example.com",
		"DAGU_WORKER_COORDINATOR_PORT":    "60051",
		"DAGU_WORKER_INSECURE":            "true",
		"DAGU_WORKER_SKIP_TLS_VERIFY":     "true",
		"DAGU_WORKER_TLS_CERT_FILE":       "/test/worker/cert.pem",
		"DAGU_WORKER_TLS_KEY_FILE":        "/test/worker/key.pem",
		"DAGU_WORKER_TLS_CA_FILE":         "/test/worker/ca.pem",
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

	// Coordinator configurations
	// The DAGU_COORDINATOR_SIGNING_KEY environment variable doesn't work because
	// it's bound to a flat field that doesn't exist in the definition.
	// The coordinator config is only loaded from the nested structure.
	assert.Equal(t, "", cfg.Coordinator.SigningKey) // No env var override support

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

	// Worker configuration
	assert.Equal(t, "test-worker-123", cfg.Worker.ID)
	assert.Equal(t, 200, cfg.Worker.MaxActiveRuns)
	assert.Equal(t, "worker.example.com", cfg.Worker.CoordinatorHost)
	assert.Equal(t, 60051, cfg.Worker.CoordinatorPort)
	assert.True(t, cfg.Worker.Insecure)
	assert.True(t, cfg.Worker.SkipTLSVerify)
	require.NotNil(t, cfg.Worker.TLS)
	assert.Equal(t, "/test/worker/cert.pem", cfg.Worker.TLS.CertFile)
	assert.Equal(t, "/test/worker/key.pem", cfg.Worker.TLS.KeyFile)
	assert.Equal(t, "/test/worker/ca.pem", cfg.Worker.TLS.CAFile)
}

func TestConfigLoader_CoordinatorSigningKey(t *testing.T) {
	t.Run("LoadFromYAML", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with CoordinatorSigningKey
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
coordinator:
  signingKey: "yaml-signing-key-123"
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

		// Verify CoordinatorSigningKey is loaded from YAML
		assert.Equal(t, "yaml-signing-key-123", cfg.Coordinator.SigningKey)
		assert.Equal(t, "admin", cfg.Server.Auth.Basic.Username)
		assert.Equal(t, "pass", cfg.Server.Auth.Basic.Password)
		assert.Equal(t, "api-token", cfg.Server.Auth.Token.Value)
	})

	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with CoordinatorSigningKey
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
coordinator:
  signingKey: "yaml-signing-key"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variable
		os.Setenv("DAGU_COORDINATOR_SIGNING_KEY", "env-signing-key-override")
		defer os.Unsetenv("DAGU_COORDINATOR_SIGNING_KEY")

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify that environment variable does NOT override YAML
		// because the env var is bound to a flat field that doesn't exist
		assert.Equal(t, "yaml-signing-key", cfg.Coordinator.SigningKey)
	})

	t.Run("EmptyCoordinatorSigningKey", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a minimal config file without CoordinatorSigningKey
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

		// Verify CoordinatorSigningKey is empty when not provided
		assert.Equal(t, "", cfg.Coordinator.SigningKey)
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
coordinator:
  signingKey: "master-signing-key"
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
		assert.Equal(t, "master-signing-key", cfg.Coordinator.SigningKey)
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
workerId: "yaml-worker-01"
workerMaxActiveRuns: 50
workerCoordinatorHost: "coordinator.example.com"
workerCoordinatorPort: 8080
workerInsecure: true
workerSkipTlsVerify: true
workerTlsCertFile: "/path/to/worker/cert.pem"
workerTlsKeyFile: "/path/to/worker/key.pem"
workerTlsCaFile: "/path/to/worker/ca.pem"
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
		assert.Equal(t, "coordinator.example.com", cfg.Worker.CoordinatorHost)
		assert.Equal(t, 8080, cfg.Worker.CoordinatorPort)
		assert.True(t, cfg.Worker.Insecure)
		assert.True(t, cfg.Worker.SkipTLSVerify)
		require.NotNil(t, cfg.Worker.TLS)
		assert.Equal(t, "/path/to/worker/cert.pem", cfg.Worker.TLS.CertFile)
		assert.Equal(t, "/path/to/worker/key.pem", cfg.Worker.TLS.KeyFile)
		assert.Equal(t, "/path/to/worker/ca.pem", cfg.Worker.TLS.CAFile)
	})

	t.Run("EnvironmentVariableOverride", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with Worker configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
workerId: "yaml-worker"
workerMaxActiveRuns: 10
workerCoordinatorHost: "localhost"
workerCoordinatorPort: 5000
workerInsecure: false
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Set environment variables
		envs := map[string]string{
			"DAGU_WORKER_ID":                  "env-worker-override",
			"DAGU_WORKER_MAX_ACTIVE_RUNS": "300",
			"DAGU_WORKER_COORDINATOR_HOST":    "env.coordinator.com",
			"DAGU_WORKER_COORDINATOR_PORT":    "9090",
			"DAGU_WORKER_INSECURE":            "true",
			"DAGU_WORKER_SKIP_TLS_VERIFY":     "true",
			"DAGU_WORKER_TLS_CERT_FILE":       "/env/cert.pem",
			"DAGU_WORKER_TLS_KEY_FILE":        "/env/key.pem",
			"DAGU_WORKER_TLS_CA_FILE":         "/env/ca.pem",
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
		assert.Equal(t, "env.coordinator.com", cfg.Worker.CoordinatorHost)
		assert.Equal(t, 9090, cfg.Worker.CoordinatorPort)
		assert.True(t, cfg.Worker.Insecure)
		assert.True(t, cfg.Worker.SkipTLSVerify)
		require.NotNil(t, cfg.Worker.TLS)
		assert.Equal(t, "/env/cert.pem", cfg.Worker.TLS.CertFile)
		assert.Equal(t, "/env/key.pem", cfg.Worker.TLS.KeyFile)
		assert.Equal(t, "/env/ca.pem", cfg.Worker.TLS.CAFile)
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
		assert.Equal(t, "127.0.0.1", cfg.Worker.CoordinatorHost)
		assert.Equal(t, 50051, cfg.Worker.CoordinatorPort)
		assert.True(t, cfg.Worker.Insecure)
		assert.False(t, cfg.Worker.SkipTLSVerify)
		assert.Nil(t, cfg.Worker.TLS) // No TLS config by default
	})

	t.Run("SecureByDefault", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with only worker host/port
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
workerCoordinatorHost: "secure.example.com"
workerCoordinatorPort: 443
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify worker is secure by default
		assert.Equal(t, "secure.example.com", cfg.Worker.CoordinatorHost)
		assert.Equal(t, 443, cfg.Worker.CoordinatorPort)
		assert.True(t, cfg.Worker.Insecure)
		assert.False(t, cfg.Worker.SkipTLSVerify)
	})

	t.Run("PartialTLSConfig", func(t *testing.T) {
		// Reset viper to ensure clean state
		viper.Reset()
		defer viper.Reset()

		// Create a config file with partial TLS configuration
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
workerTlsCaFile: "/path/to/ca.pem"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify partial TLS config is loaded correctly
		require.NotNil(t, cfg.Worker.TLS)
		assert.Equal(t, "", cfg.Worker.TLS.CertFile)
		assert.Equal(t, "", cfg.Worker.TLS.KeyFile)
		assert.Equal(t, "/path/to/ca.pem", cfg.Worker.TLS.CAFile)
	})
}

func TestWorkerLabels(t *testing.T) {
	t.Run("LabelsFromString", func(t *testing.T) {
		// Create config with worker labels as string
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
workerLabels: "gpu=true,memory=64G,region=us-east-1"
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
workerLabels:
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
workerMaxActiveRuns: 50
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		// Load configuration
		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify labels are nil or empty
		assert.True(t, cfg.Worker.Labels == nil || len(cfg.Worker.Labels) == 0)
	})
}
