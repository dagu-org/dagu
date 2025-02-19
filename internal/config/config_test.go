package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_MigrateLegacyConfig(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Config)
		validate func(*testing.T, *Config)
	}{
		{
			name: "migrate server settings",
			setup: func(cfg *Config) {
				cfg.APIBaseURL = "/legacy/api"
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.APIBaseURL != "/legacy/api" {
					t.Errorf("APIBaseURL = %v, want %v", cfg.APIBaseURL, "/legacy/api")
				}
				if cfg.APIBasePath != "/legacy/api" {
					t.Errorf("APIBasePath = %v, want %v", cfg.APIBasePath, "/legacy/api")
				}
			},
		},
		{
			name: "migrate auth settings",
			setup: func(cfg *Config) {
				cfg.IsBasicAuth = true
				cfg.BasicAuthUsername = "user"
				cfg.BasicAuthPassword = "pass"
				cfg.IsAuthToken = true
				cfg.AuthToken = "token123"
			},
			validate: func(t *testing.T, cfg *Config) {
				if !cfg.Auth.Basic.Enabled {
					t.Error("Auth.Basic.Enabled = false, want true")
				}
				if cfg.Auth.Basic.Username != "user" {
					t.Errorf("Auth.Basic.Username = %v, want user", cfg.Auth.Basic.Username)
				}
				if cfg.Auth.Basic.Password != "pass" {
					t.Errorf("Auth.Basic.Password = %v, want pass", cfg.Auth.Basic.Password)
				}
				if !cfg.Auth.Token.Enabled {
					t.Error("Auth.Token.Enabled = false, want true")
				}
				if cfg.Auth.Token.Value != "token123" {
					t.Errorf("Auth.Token.Value = %v, want token123", cfg.Auth.Token.Value)
				}
			},
		},
		{
			name: "migrate paths",
			setup: func(cfg *Config) {
				cfg.DAGs = "/dags"
				cfg.Executable = "/bin/exec"
				cfg.LogDir = "/logs"
				cfg.DataDir = "/data"
				cfg.SuspendFlagsDir = "/suspend"
				cfg.AdminLogsDir = "/admin/logs"
				cfg.BaseConfig = "/base.yaml"
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.Paths.DAGsDir != "/dags" {
					t.Errorf("Paths.DAGsDir = %v, want /dags", cfg.Paths.DAGsDir)
				}
				if cfg.Paths.Executable != "/bin/exec" {
					t.Errorf("Paths.Executable = %v, want /bin/exec", cfg.Paths.Executable)
				}
				if cfg.Paths.LogDir != "/logs" {
					t.Errorf("Paths.LogDir = %v, want /logs", cfg.Paths.LogDir)
				}
				if cfg.Paths.DataDir != "/data" {
					t.Errorf("Paths.DataDir = %v, want /data", cfg.Paths.DataDir)
				}
				if cfg.Paths.SuspendFlagsDir != "/suspend" {
					t.Errorf("Paths.SuspendFlagsDir = %v, want /suspend", cfg.Paths.SuspendFlagsDir)
				}
				if cfg.Paths.AdminLogsDir != "/admin/logs" {
					t.Errorf("Paths.AdminLogsDir = %v, want /admin/logs", cfg.Paths.AdminLogsDir)
				}
				if cfg.Paths.BaseConfig != "/base.yaml" {
					t.Errorf("Paths.BaseConfig = %v, want /base.yaml", cfg.Paths.BaseConfig)
				}
			},
		},
		{
			name: "migrate UI settings",
			setup: func(cfg *Config) {
				cfg.LogEncodingCharset = "utf-8"
				cfg.NavbarColor = "#000000"
				cfg.NavbarTitle = "Test Dashboard"
				cfg.MaxDashboardPageLimit = 50
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.UI.LogEncodingCharset != "utf-8" {
					t.Errorf("UI.LogEncodingCharset = %v, want utf-8", cfg.UI.LogEncodingCharset)
				}
				if cfg.UI.NavbarColor != "#000000" {
					t.Errorf("UI.NavbarColor = %v, want #000000", cfg.UI.NavbarColor)
				}
				if cfg.UI.NavbarTitle != "Test Dashboard" {
					t.Errorf("UI.NavbarTitle = %v, want Test Dashboard", cfg.UI.NavbarTitle)
				}
				if cfg.UI.MaxDashboardPageLimit != 50 {
					t.Errorf("UI.MaxDashboardPageLimit = %v, want 50", cfg.UI.MaxDashboardPageLimit)
				}
			},
		},
		{
			name: "clean base path",
			setup: func(cfg *Config) {
				cfg.BasePath = "//api/v1/"
			},
			validate: func(t *testing.T, cfg *Config) {
				if cfg.BasePath != "/api/v1" {
					t.Errorf("BasePath = %v, want /api/v1", cfg.BasePath)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			tt.setup(cfg)
			cfg.MigrateLegacyConfig()
			tt.validate(t, cfg)
		})
	}
}

func TestConfigLoader_Load(t *testing.T) {
	type testCase struct {
		name           string
		data           string
		setup          func(t *testing.T)
		expectedConfig *Config
	}

	tests := []testCase{
		{
			name: "Defaults",
			data: `
dagsDir: "/dags"
logsDir: "/logs"
dataDir: "/data"
suspendFlagsDir: "/suspend"
adminLogsDir: "/admin/logs"
`,
			expectedConfig: &Config{
				Host:        "127.0.0.1",
				Port:        8080,
				APIBasePath: "/api/v1",
				LogFormat:   "text",
				TZ:          "Asia/Tokyo",
				UI: UI{
					NavbarTitle:           "Dagu",
					MaxDashboardPageLimit: 100,
					LogEncodingCharset:    "utf-8",
				},
			},
		},
		{
			name: "TopLevelFields",
			data: `
dagsDir: "/dags"
logsDir: "/logs"
dataDir: "/data"
suspendFlagsDir: "/suspend"
adminLogsDir: "/admin/logs"
baseConfig: "/base.yaml"
BasePath: "/proxy"
APIBasePath: "/proxy/api/v1"
WorkDir: "/work"
Headless: true
`,
			expectedConfig: &Config{
				Host:        "127.0.0.1",
				Port:        8080,
				APIBasePath: "/proxy/api/v1",
				LogFormat:   "text",
				TZ:          "Asia/Tokyo",
				BasePath:    "/proxy",
				WorkDir:     "/work",
				Headless:    true,
				UI: UI{
					NavbarTitle:           "Dagu",
					MaxDashboardPageLimit: 100,
					LogEncodingCharset:    "utf-8",
				},
			},
		},
		{
			name: "Auth",
			data: `
dagsDir: "/dags"
logsDir: "/logs"
dataDir: "/data"
suspendFlagsDir: "/suspend"
adminLogsDir: "/admin/logs"
auth:
  basic:
    enabled: true
    username: "admin"	
    password: "password"
  token:
    enabled: true
    value: "abc123"
`,
			expectedConfig: &Config{
				Host:        "127.0.0.1",
				Port:        8080,
				APIBasePath: "/api/v1",
				LogFormat:   "text",
				TZ:          "Asia/Tokyo",
				UI: UI{
					NavbarTitle:           "Dagu",
					MaxDashboardPageLimit: 100,
					LogEncodingCharset:    "utf-8",
				},
				Auth: Auth{
					Basic: AuthBasic{
						Enabled:  true,
						Username: "admin",
						Password: "password",
					},
					Token: AuthToken{
						Enabled: true,
						Value:   "abc123",
					},
				},
			},
		},
		{
			name: "UI",
			data: `
ui:
  logEncodingCharset: "shift-jis"
  navbarColor: "#FF0000"
  navbarTitle: "Test Dashboard"
  maxDashboardPageLimit: 50
`,
			expectedConfig: &Config{
				Host:        "127.0.0.1",
				Port:        8080,
				APIBasePath: "/api/v1",
				LogFormat:   "text",
				TZ:          "Asia/Tokyo",
				UI: UI{
					NavbarTitle:           "Test Dashboard",
					NavbarColor:           "#FF0000",
					MaxDashboardPageLimit: 50,
					LogEncodingCharset:    "shift-jis",
				},
			},
		},
		{
			name: "LoadFromEnv",
			data: `
host: "127.0.0.1"
port: 8080
debug: true
auth:
  basic:
    enabled: true
    username: "admin"
    password: "secret"
ui:
  navbarTitle: "Test Dashboard"
  maxDashboardPageLimit: 50
`,
			setup: func(t *testing.T) {
				os.Setenv("DAGU_HOST", "localhost")
				os.Setenv("DAGU_PORT", "9090")
				t.Cleanup(func() {
					os.Unsetenv("DAGU_HOST")
					os.Unsetenv("DAGU_PORT")
				})
			},
			expectedConfig: &Config{
				Host:        "localhost",
				Port:        9090,
				Debug:       true,
				APIBasePath: "/api/v1",
				LogFormat:   "text",
				TZ:          "Asia/Tokyo",
				Auth: Auth{
					Basic: AuthBasic{
						Enabled:  true,
						Username: "admin",
						Password: "secret",
					},
				},
				UI: UI{
					NavbarTitle:           "Test Dashboard",
					MaxDashboardPageLimit: 50,
					LogEncodingCharset:    "utf-8",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "dagu-config-test")
			require.NoError(t, err, "failed to create temp dir: %v", err)

			os.Setenv("HOME", tmpDir)
			os.Setenv("DAGU_TZ", "Asia/Tokyo")

			t.Cleanup(func() {
				os.RemoveAll(tmpDir)
				os.Unsetenv("HOME")
				os.Unsetenv("DAGU_TZ")
			})

			if tc.setup != nil {
				tc.setup(t)
			}

			configDir := filepath.Join(tmpDir, ".config", "dagu")
			err = os.MkdirAll(configDir, 0755)
			require.NoError(t, err, "failed to create config dir: %v", err)

			configFile := filepath.Join(configDir, "config.yaml")
			testConfig := []byte(tc.data)
			err = os.WriteFile(configFile, testConfig, 0644)
			require.NoError(t, err, "failed to write config file: %v", err)

			cfg, err := Load()
			require.NoError(t, err, "failed to load config: %v", err)

			// assert.EqualValues(t, tc.expectedConfig, cfg)
			assert.Equal(t, tc.expectedConfig.Host, cfg.Host, "Host = %v, want %v", cfg.Host, tc.expectedConfig.Host)
			assert.Equal(t, tc.expectedConfig.Port, cfg.Port, "Port = %v, want %v", cfg.Port, tc.expectedConfig.Port)
			assert.Equal(t, tc.expectedConfig.Debug, cfg.Debug, "Debug = %v, want %v", cfg.Debug, tc.expectedConfig.Debug)
			assert.Equal(t, tc.expectedConfig.BasePath, cfg.BasePath, "BasePath = %v, want %v", cfg.BasePath, tc.expectedConfig.BasePath)
			assert.Equal(t, tc.expectedConfig.APIBasePath, cfg.APIBasePath, "APIBasePath = %v, want %v", cfg.APIBasePath, tc.expectedConfig.APIBasePath)
			assert.Equal(t, tc.expectedConfig.WorkDir, cfg.WorkDir, "WorkDir = %v, want %v", cfg.WorkDir, tc.expectedConfig.WorkDir)
			assert.Equal(t, tc.expectedConfig.Headless, cfg.Headless, "Headless = %v, want %v", cfg.Headless, tc.expectedConfig.Headless)
			assert.Equal(t, tc.expectedConfig.LogFormat, cfg.LogFormat, "LogFormat = %v, want %v", cfg.LogFormat, tc.expectedConfig.LogFormat)
			assert.Equal(t, tc.expectedConfig.LatestStatusToday, cfg.LatestStatusToday, "LatestStatusToday = %v, want %v", cfg.LatestStatusToday, tc.expectedConfig.LatestStatusToday)
			assert.Equal(t, tc.expectedConfig.TZ, cfg.TZ, "TZ = %v, want %v", cfg.TZ, tc.expectedConfig.TZ)
			assert.Equal(t, tc.expectedConfig.Auth, cfg.Auth, "Auth = %v, want %v", cfg.Auth, tc.expectedConfig.Auth)
			assert.Equal(t, tc.expectedConfig.UI, cfg.UI, "UI = %v, want %v", cfg.UI, tc.expectedConfig.UI)
			assert.Equal(t, tc.expectedConfig.RemoteNodes, cfg.RemoteNodes, "RemoteNodes = %v, want %v", cfg.RemoteNodes, tc.expectedConfig.RemoteNodes)
			assert.Equal(t, tc.expectedConfig.TLS, cfg.TLS, "TLS = %v, want %v", cfg.TLS, tc.expectedConfig.TLS)
		})
	}

}

func TestConfigLoader_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*Config)
		wantErr bool
	}{
		{
			name: "valid config",
			setup: func(cfg *Config) {
				cfg.Port = 8080
				cfg.Auth.Basic.Enabled = true
				cfg.Auth.Basic.Username = "user"
				cfg.Auth.Basic.Password = "pass"
				cfg.UI.MaxDashboardPageLimit = 100
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			setup: func(cfg *Config) {
				cfg.Port = 70000
			},
			wantErr: true,
		},
		{
			name: "invalid basic auth",
			setup: func(cfg *Config) {
				cfg.Port = 8080
				cfg.Auth.Basic.Enabled = true
			},
			wantErr: true,
		},
		{
			name: "invalid token auth",
			setup: func(cfg *Config) {
				cfg.Port = 8080
				cfg.Auth.Token.Enabled = true
			},
			wantErr: true,
		},
		{
			name: "invalid TLS config",
			setup: func(cfg *Config) {
				cfg.Port = 8080
				cfg.TLS = &TLSConfig{
					CertFile: "/cert.pem",
				}
			},
			wantErr: true,
		},
		{
			name: "invalid dashboard limit",
			setup: func(cfg *Config) {
				cfg.Port = 8080
				cfg.UI.MaxDashboardPageLimit = 0
			},
			wantErr: true,
		},
	}

	loader := NewConfigLoader()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{}
			tt.setup(cfg)
			err := loader.validateConfig(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfigLoader_LoadLegacyEnv(t *testing.T) {
	// Set up test environment variables
	envVars := map[string]string{
		"DAGU__ADMIN_NAVBAR_COLOR": "#FF0000",
		"DAGU__ADMIN_NAVBAR_TITLE": "Legacy Dashboard",
		"DAGU__ADMIN_PORT":         "9000",
		"DAGU__ADMIN_HOST":         "0.0.0.0",
		"DAGU__DATA":               "/data/legacy",
	}

	// Set environment variables
	for k, v := range envVars {
		os.Setenv(k, v)
	}
	defer func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	}()

	cfg := &Config{}
	loader := NewConfigLoader()
	err := loader.LoadLegacyEnv(cfg)
	if err != nil {
		t.Fatalf("LoadLegacyEnv() error = %v", err)
	}

	// Verify legacy environment variables were properly loaded
	if cfg.UI.NavbarColor != "#FF0000" {
		t.Errorf("UI.NavbarColor = %v, want #FF0000", cfg.UI.NavbarColor)
	}
	if cfg.UI.NavbarTitle != "Legacy Dashboard" {
		t.Errorf("UI.NavbarTitle = %v, want Legacy Dashboard", cfg.UI.NavbarTitle)
	}
	if cfg.Port != 9000 {
		t.Errorf("Port = %v, want 9000", cfg.Port)
	}
	if cfg.Host != "0.0.0.0" {
		t.Errorf("Host = %v, want 0.0.0.0", cfg.Host)
	}
	if cfg.Paths.DataDir != "/data/legacy" {
		t.Errorf("Paths.DataDir = %v, want /data/legacy", cfg.Paths.DataDir)
	}
}
