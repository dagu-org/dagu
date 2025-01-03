package config

import (
	"os"
	"path/filepath"
	"testing"
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
	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "dagu-config-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up test environment
	os.Setenv("HOME", tmpDir)
	os.Setenv("DAGU_HOST", "localhost")
	os.Setenv("DAGU_PORT", "9090")
	defer func() {
		os.Unsetenv("HOME")
		os.Unsetenv("DAGU_HOST")
		os.Unsetenv("DAGU_PORT")
	}()

	// Create test config directory
	configDir := filepath.Join(tmpDir, ".config", "dagu")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Create test config file
	configFile := filepath.Join(configDir, "config.yaml")
	testConfig := []byte(`
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
`)
	if err := os.WriteFile(configFile, testConfig, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify loaded configuration
	if cfg.Host != "localhost" {
		t.Errorf("Host = %v, want localhost", cfg.Host)
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %v, want 9090", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if !cfg.Auth.Basic.Enabled {
		t.Error("Auth.Basic.Enabled = false, want true")
	}
	if cfg.Auth.Basic.Username != "admin" {
		t.Errorf("Auth.Basic.Username = %v, want admin", cfg.Auth.Basic.Username)
	}
	if cfg.Auth.Basic.Password != "secret" {
		t.Errorf("Auth.Basic.Password = %v, want secret", cfg.Auth.Basic.Password)
	}
	if cfg.UI.NavbarTitle != "Test Dashboard" {
		t.Errorf("UI.NavbarTitle = %v, want Test Dashboard", cfg.UI.NavbarTitle)
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
