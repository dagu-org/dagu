package config_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_WithValidFile(t *testing.T) {
	// Reset viper between tests to avoid leakage of global state.
	viper.Reset()

	// Create a temporary config file with a valid YAML configuration.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
host: "0.0.0.0"
port: 9090
permissions:
  writeDAGs: false
  runDAGs: false
debug: true
basePath: "/dagu"
apiBasePath: "/api/v1"
tz: "UTC"
logFormat: "json"
workingDir: "/var/dagu/work"
headless: true
paths:
  dagsDir: "/var/dagu/dags"
  logDir: "/var/dagu/logs"
  dataDir: "/var/dagu/data"
  suspendFlagsDir: "/var/dagu/suspend"
  adminLogsDir: "/var/dagu/adminlogs"
  baseConfig: "/var/dagu/base.yaml"
  executable: "/usr/local/bin/dagu"
  queueDir: "/var/dagu/queue"
  procDir: "/var/dagu/proc"
ui:
  navbarTitle: "Test Dagu"
  maxDashboardPageLimit: 50
  logEncodingCharset: "utf-8"
auth:
  basic:
    enabled: true
    username: "admin"
    password: "secret"
  token:
    enabled: false
remoteNodes:
  - name: "node1"
    apiBaseURL: "http://node1.example.com/api"
tls:
  certFile: "/path/to/cert.pem"
  keyFile: "/path/to/key.pem"
scheduler:
  port: 7890
  lockStaleThreshold: 50s
  lockRetryInterval: 10s
  zombieDetectionInterval: 60s
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	// Load the configuration using the provided config file option.
	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify global settings.
	assert.Equal(t, true, cfg.Global.Debug)
	assert.Equal(t, "json", cfg.Global.LogFormat)
	assert.Equal(t, "UTC", cfg.Global.TZ)
	assert.NotNil(t, cfg.Global.Location)
	assert.Equal(t, 0, cfg.Global.TzOffsetInSec)

	// Verify server settings.
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)

	// cleanBasePath should clean the basePath as provided.
	assert.Equal(t, "/dagu", cfg.Server.BasePath)
	assert.Equal(t, "/api/v1", cfg.Server.APIBasePath)
	assert.Equal(t, true, cfg.Server.Headless)
	assert.False(t, cfg.Server.Permissions[config.PermissionWriteDAGs])
	assert.False(t, cfg.Server.Permissions[config.PermissionRunDAGs])

	// Verify authentication.
	assert.True(t, cfg.Server.Auth.Basic.Enabled())
	assert.Equal(t, "admin", cfg.Server.Auth.Basic.Username)
	assert.Equal(t, "secret", cfg.Server.Auth.Basic.Password)

	// Verify TLS configuration.
	require.NotNil(t, cfg.Server.TLS)
	assert.Equal(t, "/path/to/cert.pem", cfg.Server.TLS.CertFile)
	assert.Equal(t, "/path/to/key.pem", cfg.Server.TLS.KeyFile)

	// Verify remote nodes.
	require.Len(t, cfg.Server.RemoteNodes, 1)
	assert.Equal(t, "node1", cfg.Server.RemoteNodes[0].Name)
	assert.Equal(t, "http://node1.example.com/api", cfg.Server.RemoteNodes[0].APIBaseURL)

	// Verify paths.
	assert.Equal(t, "/var/dagu/dags", cfg.Paths.DAGsDir)
	assert.Equal(t, "/var/dagu/logs", cfg.Paths.LogDir)
	assert.Equal(t, "/var/dagu/data", cfg.Paths.DataDir)
	assert.Equal(t, "/var/dagu/suspend", cfg.Paths.SuspendFlagsDir)
	assert.Equal(t, "/var/dagu/adminlogs", cfg.Paths.AdminLogsDir)
	assert.Equal(t, "/var/dagu/base.yaml", cfg.Paths.BaseConfig)
	assert.Equal(t, "/usr/local/bin/dagu", cfg.Paths.Executable)
	assert.Equal(t, "/var/dagu/data/dag-runs", cfg.Paths.DAGRunsDir)
	assert.Equal(t, "/var/dagu/queue", cfg.Paths.QueueDir)
	assert.Equal(t, "/var/dagu/proc", cfg.Paths.ProcDir)

	// Verify scheduler settings.
	assert.Equal(t, 7890, cfg.Scheduler.Port)
	// Default scheduler lock settings should be applied
	assert.Equal(t, 50*time.Second, cfg.Scheduler.LockStaleThreshold)
	assert.Equal(t, 10*time.Second, cfg.Scheduler.LockRetryInterval)
	assert.Equal(t, 60*time.Second, cfg.Scheduler.ZombieDetectionInterval)

	// Verify new distributed execution fields have defaults
	assert.Equal(t, "", cfg.Coordinator.Host)
	assert.Equal(t, 0, cfg.Coordinator.Port)
	assert.Equal(t, "", cfg.Worker.ID)
	assert.Equal(t, 100, cfg.Worker.MaxActiveRuns) // Default is 100
	assert.Nil(t, cfg.Worker.Labels)
	assert.Equal(t, "/var/dagu/data/service-registry", cfg.Paths.ServiceRegistryDir) // Auto-generated from dataDir

	// Verify UI settings.
	assert.Equal(t, "Test Dagu", cfg.UI.NavbarTitle)
	assert.Equal(t, 50, cfg.UI.MaxDashboardPageLimit)
	assert.Equal(t, "utf-8", cfg.UI.LogEncodingCharset)

	// Verify DAGs defaults (not specified in config)
	assert.Equal(t, "name", cfg.UI.DAGs.SortField)
	assert.Equal(t, "asc", cfg.UI.DAGs.SortOrder)
}

func TestLoadConfig_Defaults(t *testing.T) {
	viper.Reset()
	// When no config file is provided, the defaults should be applied.
	cfg, err := config.Load(config.WithConfigFile(test.TestdataPath(t, "config/empty.yaml")))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// According to setDefaultValues, defaults should be:
	// host: "127.0.0.1", port: 8080, debug: false, logFormat: "text"
	assert.Equal(t, "127.0.0.1", cfg.Server.Host)
	assert.Equal(t, 8080, cfg.Server.Port)
	assert.False(t, cfg.Global.Debug)
	assert.Equal(t, "text", cfg.Global.LogFormat)

	// For UI defaults, maxDashboardPageLimit should be 100 and logEncodingCharset "utf-8".
	assert.Equal(t, 100, cfg.UI.MaxDashboardPageLimit)
	assert.Equal(t, "utf-8", cfg.UI.LogEncodingCharset)

	// DAGs defaults
	assert.Equal(t, "name", cfg.UI.DAGs.SortField)
	assert.Equal(t, "asc", cfg.UI.DAGs.SortOrder)

	// Permissions should be set to true
	assert.True(t, cfg.Server.Permissions[config.PermissionWriteDAGs])
	assert.True(t, cfg.Server.Permissions[config.PermissionRunDAGs])

	// Scheduler defaults
	assert.Equal(t, 8090, cfg.Scheduler.Port)
	assert.Equal(t, 30*time.Second, cfg.Scheduler.LockStaleThreshold)
	assert.Equal(t, 5*time.Second, cfg.Scheduler.LockRetryInterval)
	assert.Equal(t, 45*time.Second, cfg.Scheduler.ZombieDetectionInterval)

	// Worker defaults
	assert.Equal(t, 100, cfg.Worker.MaxActiveRuns)

	// Peer defaults
	assert.True(t, cfg.Global.Peer.Insecure)
}

func TestValidateConfig_BasicAuthError(t *testing.T) {
	viper.Reset()
	// Create a temporary config file where basic auth is enabled but username is missing.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
auth:
  basic:
    username: ""
    password: "secret"
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	_, err = config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
}

func TestValidateConfig_OIDCError(t *testing.T) {
	viper.Reset()
	// Create a temporary config file where basic auth is enabled but username is missing.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
auth:
  oidc:
    clientId: "example-app"
    clientSecret: "example-secret"
    clientUrl: "http://127.0.0.1:8080"
    issuer: "http://127.0.0.1:5556/dex"
    scopes:
      - openid
      - profile
      - email
    whitelist:
      - admin@example.com
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	_, err = config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
}

func TestValidateConfig_TLSError(t *testing.T) {
	viper.Reset()
	// Create a config file with incomplete TLS settings.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
tls:
  certFile: "/path/to/cert.pem"
  keyFile: ""
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	_, err = config.Load(config.WithConfigFile(configFile))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TLS configuration incomplete")
}

func TestLoadLegacyEnv_AllCases(t *testing.T) {
	// Reset viper to ensure a clean state.
	viper.Reset()

	// Define a slice of test cases, each corresponding to one legacy environment variable.
	tests := []struct {
		envVar string
		value  string
		verify func(cfg *config.Config, t *testing.T)
	}{
		{
			envVar: "DAGU__ADMIN_NAVBAR_COLOR",
			value:  "blue",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "blue", cfg.UI.NavbarColor, "expected legacy navbar color to be blue")
			},
		},
		{
			envVar: "DAGU__ADMIN_NAVBAR_TITLE",
			value:  "MyLegacyTitle",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "MyLegacyTitle", cfg.UI.NavbarTitle, "expected legacy navbar title to be MyLegacyTitle")
			},
		},
		{
			envVar: "DAGU__ADMIN_PORT",
			value:  "1234",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, 1234, cfg.Server.Port, "expected legacy port to be 1234")
			},
		},
		{
			envVar: "DAGU__ADMIN_HOST",
			value:  "0.0.0.0",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "0.0.0.0", cfg.Server.Host, "expected legacy host to be 0.0.0.0")
			},
		},
		{
			envVar: "DAGU__DATA",
			value:  "/data/legacy",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "/data/legacy", cfg.Paths.DataDir, "expected legacy data directory to be /data/legacy")
			},
		},
		{
			envVar: "DAGU__SUSPEND_FLAGS_DIR",
			value:  "/suspend/legacy",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "/suspend/legacy", cfg.Paths.SuspendFlagsDir, "expected legacy suspend flags directory to be /suspend/legacy")
			},
		},
		{
			envVar: "DAGU__ADMIN_LOGS_DIR",
			value:  "/admin/legacy",
			verify: func(cfg *config.Config, t *testing.T) {
				assert.Equal(t, "/admin/legacy", cfg.Paths.AdminLogsDir, "expected legacy admin logs directory to be /admin/legacy")
			},
		},
	}

	// Set all legacy environment variables.
	for _, tc := range tests {
		err := os.Setenv(tc.envVar, tc.value)
		require.NoError(t, err, "failed to set environment variable %s", tc.envVar)
		defer os.Unsetenv(tc.envVar)
	}

	// Load the configuration.
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Run each verification function to ensure the legacy env values were properly loaded.
	for _, tc := range tests {
		tc.verify(cfg, t)
	}
}

func TestSetExecutable(t *testing.T) {
	viper.Reset()
	// Create a config file that leaves the executable path empty.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
paths:
  executable: ""
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	// The setExecutable function should fill in the executable path.
	assert.NotEmpty(t, cfg.Paths.Executable)
}

func TestCleanBasePath(t *testing.T) {
	viper.Reset()
	// Create a config file with an unclean basePath.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
basePath: "////dagu//"
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)
	// Expect cleanBasePath to produce "/dagu" from "////dagu//"
	assert.Equal(t, "/dagu", cfg.Server.BasePath)
}

// TestLoadLegacyFields_AllSet verifies that when legacy fields are set in the config definition,
// they properly override or populate the corresponding fields in the Config.
func TestLoadLegacyFields_AllSet(t *testing.T) {
	loader := &config.ConfigLoader{}

	// Create a sample configDef with all legacy fields populated.
	def := config.Definition{
		BasicAuthUsername:     "legacyUser",
		BasicAuthPassword:     "legacyPass",
		IsAuthToken:           true,
		AuthToken:             "legacyToken",
		DAGs:                  "legacyDags",
		DAGsDir:               "/usr/user/dags", // should override def.DAGs if both are set
		Executable:            "/usr/bin/legacy",
		LogDir:                "/var/log/legacy",
		DataDir:               "/var/data/legacy",
		SuspendFlagsDir:       "/var/suspend/legacy",
		AdminLogsDir:          "/var/admin/legacy",
		BaseConfig:            "/etc/legacy/base.yaml",
		LogEncodingCharset:    "latin1",
		NavbarColor:           "red",
		NavbarTitle:           "Legacy Dagu",
		MaxDashboardPageLimit: 42,
	}

	// Initialize a Config instance with zero (or default) values.
	cfg := config.Config{
		Server: config.Server{
			Auth: config.Auth{
				Basic: config.AuthBasic{},
				Token: config.AuthToken{},
			},
		},
		Paths: config.PathsConfig{},
		UI:    config.UI{},
	}

	loader.LoadLegacyFields(&cfg, def)

	// Check that legacy fields are correctly assigned.
	assert.Equal(t, "legacyUser", cfg.Server.Auth.Basic.Username, "BasicAuthUsername should be set")
	assert.Equal(t, "legacyPass", cfg.Server.Auth.Basic.Password, "BasicAuthPassword should be set")
	assert.True(t, cfg.Server.Auth.Basic.Enabled(), "Basic auth should be enabled when BasicAuthUsername is set")
	assert.True(t, cfg.Server.Auth.Token.Enabled(), "Auth token should be enabled when IsAuthToken is true")
	assert.True(t, cfg.Server.Auth.Token.Enabled(), "Auth token should be enabled")
	assert.Equal(t, "legacyToken", cfg.Server.Auth.Token.Value, "Auth token value should be set")
	// When both DAGs and DAGsDir are provided, DAGsDir takes precedence.
	assert.Equal(t, "/usr/user/dags", cfg.Paths.DAGsDir, "DAGsDir should be set from def.DAGsDir")
	assert.Equal(t, "/usr/bin/legacy", cfg.Paths.Executable, "Executable should be set")
	assert.Equal(t, "/var/log/legacy", cfg.Paths.LogDir, "LogDir should be set")
	assert.Equal(t, "/var/data/legacy", cfg.Paths.DataDir, "DataDir should be set")
	assert.Equal(t, "/var/suspend/legacy", cfg.Paths.SuspendFlagsDir, "SuspendFlagsDir should be set")
	assert.Equal(t, "/var/admin/legacy", cfg.Paths.AdminLogsDir, "AdminLogsDir should be set")
	assert.Equal(t, "/etc/legacy/base.yaml", cfg.Paths.BaseConfig, "BaseConfig should be set")
	assert.Equal(t, "latin1", cfg.UI.LogEncodingCharset, "LogEncodingCharset should be set")
	assert.Equal(t, "red", cfg.UI.NavbarColor, "NavbarColor should be set")
	assert.Equal(t, "Legacy Dagu", cfg.UI.NavbarTitle, "NavbarTitle should be set")
	assert.Equal(t, 42, cfg.UI.MaxDashboardPageLimit, "MaxDashboardPageLimit should be set")
}

// TestLoadLegacyFields_NoneSet verifies that if none of the legacy fields are set in the config definition,
// the Config instance remains unchanged.
func TestLoadLegacyFields_NoneSet(t *testing.T) {
	loader := &config.ConfigLoader{}
	def := config.Definition{} // All legacy fields are zero values.

	// Create a Config instance with preset (non-zero) values.
	cfg := config.Config{
		Server: config.Server{
			Auth: config.Auth{
				Basic: config.AuthBasic{
					Username: "presetUser",
					Password: "presetPass",
				},
				Token: config.AuthToken{
					Value: "presetToken",
				},
			},
		},
		Paths: config.PathsConfig{
			DAGsDir:         "presetDags",
			Executable:      "/usr/bin/preset",
			LogDir:          "presetLog",
			DataDir:         "presetData",
			SuspendFlagsDir: "presetSuspend",
			AdminLogsDir:    "presetAdmin",
			BaseConfig:      "presetBase",
		},
		UI: config.UI{
			LogEncodingCharset:    "utf-8",
			NavbarColor:           "green",
			NavbarTitle:           "Preset Dagu",
			MaxDashboardPageLimit: 100,
		},
	}

	loader.LoadLegacyFields(&cfg, def)

	// Verify that none of the preset values have been overwritten.
	assert.Equal(t, "presetUser", cfg.Server.Auth.Basic.Username)
	assert.Equal(t, "presetPass", cfg.Server.Auth.Basic.Password)
	assert.Equal(t, "presetToken", cfg.Server.Auth.Token.Value)
	assert.Equal(t, "presetDags", cfg.Paths.DAGsDir)
	assert.Equal(t, "/usr/bin/preset", cfg.Paths.Executable)
	assert.Equal(t, "presetLog", cfg.Paths.LogDir)
	assert.Equal(t, "presetData", cfg.Paths.DataDir)
	assert.Equal(t, "presetSuspend", cfg.Paths.SuspendFlagsDir)
	assert.Equal(t, "presetAdmin", cfg.Paths.AdminLogsDir)
	assert.Equal(t, "presetBase", cfg.Paths.BaseConfig)
	assert.Equal(t, "utf-8", cfg.UI.LogEncodingCharset)
	assert.Equal(t, "green", cfg.UI.NavbarColor)
	assert.Equal(t, "Preset Dagu", cfg.UI.NavbarTitle)
	assert.Equal(t, 100, cfg.UI.MaxDashboardPageLimit)
}

// TestSetTimezone tests the setTimezone method of the Global struct.
func TestSetTimezone(t *testing.T) {
	// Save the original TZ environment variable to restore it later
	originalTZ := os.Getenv("TZ")
	defer os.Setenv("TZ", originalTZ)

	// Test case 1: When TZ is set to a valid timezone
	t.Run("ValidTimezone", func(t *testing.T) {
		viper.Reset()
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
tz: "America/New_York"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// Verify that the timezone was set correctly
		assert.Equal(t, "America/New_York", cfg.Global.TZ)
		assert.NotNil(t, cfg.Global.Location)
		assert.Equal(t, "America/New_York", cfg.Global.Location.String())

		// Verify that the TZ environment variable was set
		assert.Equal(t, "America/New_York", os.Getenv("TZ"))

		// Verify that the offset was calculated correctly
		// Note: This test might fail during daylight saving time changes
		// We'll check that the offset is a non-empty string
		assert.NotEmpty(t, cfg.Global.TzOffsetInSec)

		// Get the expected offset for verification
		nyLoc, _ := time.LoadLocation("America/New_York")
		currentTime := time.Now().In(nyLoc)
		_, expectedOffset := currentTime.Zone()

		assert.Equal(t, expectedOffset, cfg.Global.TzOffsetInSec)
	})

	// Test case 2: When TZ is set to an invalid timezone
	t.Run("InvalidTimezone", func(t *testing.T) {
		viper.Reset()
		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
tz: "NonExistentTimezone"
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		_, err = config.Load(config.WithConfigFile(configFile))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	// Test case 3: When TZ is not set
	t.Run("NoTimezone", func(t *testing.T) {
		viper.Reset()
		// Unset the TZ environment variable to ensure it doesn't affect the test
		os.Unsetenv("TZ")

		tempDir := t.TempDir()
		configFile := filepath.Join(tempDir, "config.yaml")
		configContent := `
# No TZ setting
`
		err := os.WriteFile(configFile, []byte(configContent), 0600)
		require.NoError(t, err)

		cfg, err := config.Load(config.WithConfigFile(configFile))
		require.NoError(t, err)
		require.NotNil(t, cfg)

		// When TZ is not set, it should use the local timezone
		// If offset is 0, it should set TZ to "UTC"
		// Otherwise, it should set TZ to "UTC+X" where X is the offset in hours

		_, expectedOffset := time.Now().Zone()
		if expectedOffset == 0 {
			assert.Equal(t, "UTC", cfg.Global.TZ)
			assert.Equal(t, 0, cfg.Global.TzOffsetInSec)
		} else {
			expectedTZ := fmt.Sprintf("UTC%+d", expectedOffset/3600)
			assert.Equal(t, expectedTZ, cfg.Global.TZ)
			assert.Equal(t, expectedOffset, cfg.Global.TzOffsetInSec)
		}

		// Verify that the Location is set to time.Local
		assert.Equal(t, time.Local, cfg.Global.Location)
	})
}

func TestLoadConfig_WithQueueConfiguration(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
queues:
  enabled: true
  config:
    - name: "globalQueue"
      maxActiveRuns: 5
    - name: "highPriorityQueue"
      maxActiveRuns: 2
    - name: "lowPriorityQueue"
      maxActiveRuns: 10
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify queue configuration is loaded correctly
	assert.True(t, cfg.Queues.Enabled)
	assert.Len(t, cfg.Queues.Config, 3)

	// Check specific queue configurations
	globalQueue := cfg.Queues.Config[0]
	assert.Equal(t, "globalQueue", globalQueue.Name)
	assert.Equal(t, 5, globalQueue.MaxActiveRuns)

	highPriorityQueue := cfg.Queues.Config[1]
	assert.Equal(t, "highPriorityQueue", highPriorityQueue.Name)
	assert.Equal(t, 2, highPriorityQueue.MaxActiveRuns)

	lowPriorityQueue := cfg.Queues.Config[2]
	assert.Equal(t, "lowPriorityQueue", lowPriorityQueue.Name)
	assert.Equal(t, 10, lowPriorityQueue.MaxActiveRuns)
}

func TestLoadConfig_WithQueueDisabled(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
queues:
  enabled: false
  config:
    - name: "testQueue"
      maxActiveRuns: 3
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify queue is disabled but config is still loaded
	assert.False(t, cfg.Queues.Enabled)
	assert.Len(t, cfg.Queues.Config, 1)
	assert.Equal(t, "testQueue", cfg.Queues.Config[0].Name)
	assert.Equal(t, 3, cfg.Queues.Config[0].MaxActiveRuns)
}

func TestLoadConfig_DefaultQueueConfiguration(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
# No queue configuration specified
debug: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify default queue configuration
	assert.True(t, cfg.Queues.Enabled)  // Should default to enabled
	assert.Len(t, cfg.Queues.Config, 0) // No queue configs by default
}

func TestLoadConfig_QueueEnvironmentOverride(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
queues:
  enabled: true
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set environment variable to override
	originalEnv := os.Getenv("DAGU_QUEUE_ENABLED")
	defer os.Setenv("DAGU_QUEUE_ENABLED", originalEnv)

	os.Setenv("DAGU_QUEUE_ENABLED", "false")

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify environment variable overrides config file
	assert.False(t, cfg.Queues.Enabled)
}

func TestLoadConfig_WithDAGsConfiguration(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
ui:
  dags:
    sortField: "lastRun"
    sortOrder: "desc"
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify DAGs configuration
	assert.Equal(t, "lastRun", cfg.UI.DAGs.SortField)
	assert.Equal(t, "desc", cfg.UI.DAGs.SortOrder)
}

func TestLoadConfig_DAGsEnvironmentOverride(t *testing.T) {
	viper.Reset()

	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
ui:
  dags:
    sortField: "name"
    sortOrder: "asc"
`
	err := os.WriteFile(configFile, []byte(configContent), 0600)
	require.NoError(t, err)

	// Set environment variables to override
	originalField := os.Getenv("DAGU_UI_DAGS_SORT_FIELD")
	originalOrder := os.Getenv("DAGU_UI_DAGS_SORT_ORDER")
	defer os.Setenv("DAGU_UI_DAGS_SORT_FIELD", originalField)
	defer os.Setenv("DAGU_UI_DAGS_SORT_ORDER", originalOrder)

	os.Setenv("DAGU_UI_DAGS_SORT_FIELD", "status")
	os.Setenv("DAGU_UI_DAGS_SORT_ORDER", "desc")

	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify environment variables override config file
	assert.Equal(t, "status", cfg.UI.DAGs.SortField)
	assert.Equal(t, "desc", cfg.UI.DAGs.SortOrder)
}
