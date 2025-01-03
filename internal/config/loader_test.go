package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/spf13/viper"
)

var (
	testMutex sync.Mutex
)

func resetViper() {
	viper.Reset()
}

func setupTestEnv(t *testing.T) string {
	t.Helper()

	testMutex.Lock()
	t.Cleanup(func() {
		resetViper()
		testMutex.Unlock()
	})

	// Create temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "dagu-loader-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	// Create config directory structure
	configDir := filepath.Join(tmpDir, ".config", "dagu")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	// Store original environment
	originalHome := os.Getenv("HOME")
	originalTZ := os.Getenv("TZ")

	// Set test environment
	os.Setenv("HOME", tmpDir)

	// Return cleanup function
	cleanup := func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("TZ", originalTZ)
		os.RemoveAll(tmpDir)
	}

	t.Cleanup(cleanup)

	return tmpDir
}

func TestConfigLoader_SetupViper(t *testing.T) {
	tmpDir := setupTestEnv(t)

	loader := NewConfigLoader()
	err := loader.setupViper()
	if err != nil {
		t.Fatalf("setupViper() error = %v", err)
	}

	// Create test config file after viper setup
	configFile := filepath.Join(tmpDir, ".config", "dagu", "config.yaml")
	testConfig := []byte(`
host: "test-host"
port: 9999
debug: true
`)
	if err := os.WriteFile(configFile, testConfig, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Host != "test-host" {
		t.Errorf("Host = %v, want test-host", cfg.Host)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port = %v, want 9999", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
}

func TestConfigLoader_SetTimezone(t *testing.T) {
	// No need for mutex here as it doesn't use Viper
	tests := []struct {
		name    string
		tz      string
		wantErr bool
	}{
		{
			name:    "valid timezone",
			tz:      "UTC",
			wantErr: false,
		},
		{
			name:    "another valid timezone",
			tz:      "America/New_York",
			wantErr: false,
		},
		{
			name:    "invalid timezone",
			tz:      "Invalid/Timezone",
			wantErr: true,
		},
		{
			name:    "empty timezone defaults to local",
			tz:      "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := NewConfigLoader()
			cfg := &Config{TZ: tt.tz}

			err := loader.setTimezone(cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("setTimezone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if cfg.Location == nil {
					t.Error("Location is nil, want non-nil")
				}
				if tt.tz != "" && cfg.TZ != tt.tz {
					t.Errorf("TZ = %v, want %v", cfg.TZ, tt.tz)
				}
			}
		})
	}
}

func TestConfigLoader_SetExecutable(t *testing.T) {
	// No need for mutex here as it doesn't use Viper
	loader := NewConfigLoader()

	t.Run("executable not set", func(t *testing.T) {
		cfg := &Config{}
		err := loader.setExecutable(cfg)
		if err != nil {
			t.Fatalf("setExecutable() error = %v", err)
		}
		if cfg.Paths.Executable == "" {
			t.Error("Executable path is empty")
		}
	})

	t.Run("executable already set", func(t *testing.T) {
		predefinedPath := "/usr/local/bin/dagu"
		cfg := &Config{
			Paths: PathsConfig{
				Executable: predefinedPath,
			},
		}
		err := loader.setExecutable(cfg)
		if err != nil {
			t.Fatalf("setExecutable() error = %v", err)
		}
		if cfg.Paths.Executable != predefinedPath {
			t.Errorf("Executable = %v, want %v", cfg.Paths.Executable, predefinedPath)
		}
	})
}

func TestConfigLoader_BindEnvironmentVariables(t *testing.T) {
	_ = setupTestEnv(t)

	envVars := map[string]string{
		"DAGU_HOST":                "env-host",
		"DAGU_PORT":                "8888",
		"DAGU_DEBUG":               "true",
		"DAGU_BASE_PATH":           "/custom",
		"DAGU_AUTH_BASIC_ENABLED":  "true",
		"DAGU_AUTH_BASIC_USERNAME": "env-user",
		"DAGU_AUTH_BASIC_PASSWORD": "env-pass",
		"DAGU_UI_NAVBAR_TITLE":     "Env Title",
	}

	// Set environment variables
	for k, v := range envVars {
		os.Setenv(k, v)
	}

	t.Cleanup(func() {
		for k := range envVars {
			os.Unsetenv(k)
		}
	})

	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify environment variables were properly bound
	if cfg.Host != "env-host" {
		t.Errorf("Host = %v, want env-host", cfg.Host)
	}
	if cfg.Port != 8888 {
		t.Errorf("Port = %v, want 8888", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if cfg.BasePath != "/custom" {
		t.Errorf("BasePath = %v, want /custom", cfg.BasePath)
	}
	if !cfg.Auth.Basic.Enabled {
		t.Error("Auth.Basic.Enabled = false, want true")
	}
	if cfg.Auth.Basic.Username != "env-user" {
		t.Errorf("Auth.Basic.Username = %v, want env-user", cfg.Auth.Basic.Username)
	}
	if cfg.Auth.Basic.Password != "env-pass" {
		t.Errorf("Auth.Basic.Password = %v, want env-pass", cfg.Auth.Basic.Password)
	}
	if cfg.UI.NavbarTitle != "Env Title" {
		t.Errorf("UI.NavbarTitle = %v, want Env Title", cfg.UI.NavbarTitle)
	}
}

func TestConfigLoader_DefaultValues(t *testing.T) {
	_ = setupTestEnv(t)

	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify default values
	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host = %v, want 127.0.0.1", cfg.Host)
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %v, want 8080", cfg.Port)
	}
	if cfg.Debug {
		t.Error("Debug = true, want false")
	}
	if cfg.UI.MaxDashboardPageLimit != 100 {
		t.Errorf("UI.MaxDashboardPageLimit = %v, want 100", cfg.UI.MaxDashboardPageLimit)
	}
	if cfg.UI.LogEncodingCharset != "utf-8" {
		t.Errorf("UI.LogEncodingCharset = %v, want utf-8", cfg.UI.LogEncodingCharset)
	}
}

func TestConfigLoader_ConfigFileOverride(t *testing.T) {
	tmpDir := setupTestEnv(t)

	// Create config file with custom values
	configFile := filepath.Join(tmpDir, ".config", "dagu", "config.yaml")
	testConfig := []byte(`
host: "custom-host"
port: 7777
debug: true
ui:
  navbarTitle: "Custom Title"
  maxDashboardPageLimit: 200
`)
	if err := os.WriteFile(configFile, testConfig, 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	loader := NewConfigLoader()
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Verify config file values override defaults
	if cfg.Host != "custom-host" {
		t.Errorf("Host = %v, want custom-host", cfg.Host)
	}
	if cfg.Port != 7777 {
		t.Errorf("Port = %v, want 7777", cfg.Port)
	}
	if !cfg.Debug {
		t.Error("Debug = false, want true")
	}
	if cfg.UI.NavbarTitle != "Custom Title" {
		t.Errorf("UI.NavbarTitle = %v, want Custom Title", cfg.UI.NavbarTitle)
	}
	if cfg.UI.MaxDashboardPageLimit != 200 {
		t.Errorf("UI.MaxDashboardPageLimit = %v, want 200", cfg.UI.MaxDashboardPageLimit)
	}
}
