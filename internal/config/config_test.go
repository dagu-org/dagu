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

func TestLoadConfig_WithValidFile(t *testing.T) {
	// Reset viper between tests to avoid leakage of global state.
	viper.Reset()

	// Create a temporary config file with a valid YAML configuration.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
host: "0.0.0.0"
port: 9090
debug: true
basePath: "/dagu"
apiBasePath: "/api/v1"
tz: "UTC"
logFormat: "json"
workDir: "/var/dagu/work"
headless: true
paths:
  dagsDir: "/var/dagu/dags"
  logDir: "/var/dagu/logs"
  dataDir: "/var/dagu/data"
  suspendFlagsDir: "/var/dagu/suspend"
  adminLogsDir: "/var/dagu/adminlogs"
  baseConfig: "/var/dagu/base.yaml"
  executable: "/usr/local/bin/dagu"
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
`
	err := os.WriteFile(configFile, []byte(configContent), 0640)
	require.NoError(t, err)

	// Load the configuration using the provided config file option.
	cfg, err := config.Load(config.WithConfigFile(configFile))
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify global settings.
	assert.Equal(t, true, cfg.Global.Debug)
	assert.Equal(t, "json", cfg.Global.LogFormat)
	assert.Equal(t, "UTC", cfg.Global.TZ)
	assert.Equal(t, "/var/dagu/work", cfg.Global.WorkDir)
	assert.NotNil(t, cfg.Global.Location)

	// Verify server settings.
	assert.Equal(t, "0.0.0.0", cfg.Server.Host)
	assert.Equal(t, 9090, cfg.Server.Port)
	// cleanBasePath should clean the basePath as provided.
	assert.Equal(t, "/dagu", cfg.Server.BasePath)
	assert.Equal(t, "/api/v1", cfg.Server.APIBasePath)
	assert.Equal(t, true, cfg.Server.Headless)

	// Verify authentication.
	assert.True(t, cfg.Server.Auth.Basic.Enabled)
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

	// Verify UI settings.
	assert.Equal(t, "Test Dagu", cfg.UI.NavbarTitle)
	assert.Equal(t, 50, cfg.UI.MaxDashboardPageLimit)
	assert.Equal(t, "utf-8", cfg.UI.LogEncodingCharset)
}

func TestLoadConfig_Defaults(t *testing.T) {
	viper.Reset()
	// When no config file is provided, the defaults should be applied.
	cfg, err := config.Load()
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
}

func TestValidateConfig_BasicAuthError(t *testing.T) {
	viper.Reset()
	// Create a temporary config file where basic auth is enabled but username is missing.
	tempDir := t.TempDir()
	configFile := filepath.Join(tempDir, "config.yaml")
	configContent := `
auth:
  basic:
    enabled: true
    username: ""
    password: "secret"
`
	err := os.WriteFile(configFile, []byte(configContent), 0640)
	require.NoError(t, err)

	_, err = config.Load(config.WithConfigFile(configFile))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "basic auth enabled but username or password is not set")
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
	err := os.WriteFile(configFile, []byte(configContent), 0640)
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
	err := os.WriteFile(configFile, []byte(configContent), 0640)
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
	err := os.WriteFile(configFile, []byte(configContent), 0640)
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
	assert.True(t, cfg.Server.Auth.Token.Enabled, "Auth token should be enabled when IsAuthToken is true")
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
					Enabled: false,
					Value:   "presetToken",
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
	assert.False(t, cfg.Server.Auth.Token.Enabled)
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
