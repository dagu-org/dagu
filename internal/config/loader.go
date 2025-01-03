package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/viper"
)

type ConfigLoader struct {
	lock sync.Mutex
}

func NewConfigLoader() *ConfigLoader {
	return &ConfigLoader{}
}

func (l *ConfigLoader) Load() (*Config, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if err := l.setupViper(); err != nil {
		return nil, fmt.Errorf("viper setup failed: %w", err)
	}

	var cfg Config
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	// Backward compatibility for 'admin.yaml' renamed to 'config.yaml'
	viper.SetConfigName("admin")
	if err := viper.MergeInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read admin config: %w", err)
		}
	}

	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Set executable path if not already set
	if err := l.setExecutable(&cfg); err != nil {
		return nil, fmt.Errorf("failed to set executable: %w", err)
	}

	// Set timezone configuration
	if err := l.setTimezone(&cfg); err != nil {
		return nil, fmt.Errorf("failed to set timezone: %w", err)
	}

	// Validate the configuration
	if err := l.validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (l *ConfigLoader) setupViper() error {
	homeDir, err := l.getHomeDir()
	if err != nil {
		return err
	}
	xdgConfig := l.getXDGConfig(homeDir)
	resolver := newResolver("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgConfig)

	l.configureViper(resolver)
	l.bindEnvironmentVariables()
	l.setDefaultValues(resolver)

	return l.setExecutableDefault()
}

func (l *ConfigLoader) getHomeDir() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return dir, nil
}

func (l *ConfigLoader) getXDGConfig(homeDir string) XDGConfig {
	return XDGConfig{
		DataHome:   xdg.DataHome,
		ConfigHome: filepath.Join(homeDir, ".config"),
	}
}

func (l *ConfigLoader) configureViper(resolver PathResolver) {
	viper.AddConfigPath(resolver.ConfigDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("config")
	viper.SetEnvPrefix(strings.ToUpper(build.Slug))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()
}

func (l *ConfigLoader) setDefaultValues(resolver PathResolver) {
	// File paths
	viper.SetDefault("workDir", "") // Should default to DAG location
	viper.SetDefault("paths.dagsDir", resolver.DAGsDir)
	viper.SetDefault("paths.suspendFlagsDir", resolver.SuspendFlagsDir)
	viper.SetDefault("paths.dataDir", resolver.DataDir)
	viper.SetDefault("paths.logDir", resolver.LogsDir)
	viper.SetDefault("paths.adminLogsDir", resolver.AdminLogsDir)
	viper.SetDefault("paths.baseConfig", resolver.BaseConfigFile)

	// Server settings
	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", 8080)
	viper.SetDefault("debug", false)
	viper.SetDefault("basePath", "")
	viper.SetDefault("apiBaseURL", "/api/v1")
	viper.SetDefault("latestStatusToday", false)

	// UI settings
	viper.SetDefault("ui.navbarTitle", build.AppName)
	viper.SetDefault("ui.maxDashboardPageLimit", 100)
	viper.SetDefault("ui.logEncodingCharset", "utf-8")

	// Logging settings
	viper.SetDefault("logFormat", "text")
}

func (l *ConfigLoader) bindEnvironmentVariables() {
	// Server configurations
	l.bindEnv("logFormat", "LOG_FORMAT")
	l.bindEnv("basePath", "BASE_PATH")
	l.bindEnv("apiBaseURL", "API_BASE_URL")
	l.bindEnv("tz", "TZ")
	l.bindEnv("host", "HOST")
	l.bindEnv("port", "PORT")
	l.bindEnv("debug", "DEBUG")

	// UI configurations
	l.bindEnv("ui.maxDashboardPageLimit", "UI_MAX_DASHBOARD_PAGE_LIMIT")
	l.bindEnv("ui.logEncodingCharset", "UI_LOG_ENCODING_CHARSET")
	l.bindEnv("ui.navbarColor", "UI_NAVBAR_COLOR")
	l.bindEnv("ui.navbarTitle", "UI_NAVBAR_TITLE")

	// UI configurations (legacy)
	l.bindEnv("ui.maxDashboardPageLimit", "MAX_DASHBOARD_PAGE_LIMIT")
	l.bindEnv("ui.logEncodingCharset", "LOG_ENCODING_CHARSET")
	l.bindEnv("ui.navbarColor", "NAVBAR_COLOR")
	l.bindEnv("ui.navbarTitle", "NAVBAR_TITLE")

	// Authentication configurations
	l.bindEnv("auth.basic.enabled", "AUTH_BASIC_ENABLED")
	l.bindEnv("auth.basic.username", "AUTH_BASIC_USERNAME")
	l.bindEnv("auth.basic.password", "AUTH_BASIC_PASSWORD")
	l.bindEnv("auth.token.enabled", "AUTH_TOKEN_ENABLED")
	l.bindEnv("auth.token.value", "AUTH_TOKEN")

	// Authentication configurations (legacy)
	l.bindEnv("auth.basic.enabled", "IS_BASICAUTH")
	l.bindEnv("auth.basic.username", "BASICAUTH_USERNAME")
	l.bindEnv("auth.basic.password", "BASICAUTH_PASSWORD")
	l.bindEnv("auth.token.enabled", "IS_AUTHTOKEN")
	l.bindEnv("auth.token.value", "AUTHTOKEN")

	// TLS configurations
	l.bindEnv("tls.certFile", "CERT_FILE")
	l.bindEnv("tls.keyFile", "KEY_FILE")

	// File paths
	l.bindEnv("dags", "DAGS")
	l.bindEnv("dags", "DAGS_DIR")
	l.bindEnv("workDir", "WORK_DIR")
	l.bindEnv("baseConfig", "BASE_CONFIG")
	l.bindEnv("logDir", "LOG_DIR")
	l.bindEnv("dataDir", "DATA_DIR")
	l.bindEnv("suspendFlagsDir", "SUSPEND_FLAGS_DIR")
	l.bindEnv("adminLogsDir", "ADMIN_LOG_DIR")
	l.bindEnv("executable", "EXECUTABLE")

	// UI customization
	l.bindEnv("latestStatusToday", "LATEST_STATUS_TODAY")
}

func (l *ConfigLoader) bindEnv(key, env string) {
	prefix := strings.ToUpper(build.Slug) + "_"
	_ = viper.BindEnv(key, prefix+env)
}

func (l *ConfigLoader) LoadLegacyEnv(cfg *Config) error {
	legacyEnvs := map[string]struct {
		newKey string
		setter func(*Config, string)
	}{
		"DAGU__ADMIN_NAVBAR_COLOR": {
			newKey: "DAGU_NAVBAR_COLOR",
			setter: func(c *Config, v string) { c.UI.NavbarColor = v },
		},
		"DAGU__ADMIN_NAVBAR_TITLE": {
			newKey: "DAGU_NAVBAR_TITLE",
			setter: func(c *Config, v string) { c.UI.NavbarTitle = v },
		},
		"DAGU__ADMIN_PORT": {
			newKey: "DAGU_PORT",
			setter: func(c *Config, v string) {
				if i, err := strconv.Atoi(v); err == nil {
					c.Port = i
				}
			},
		},
		"DAGU__ADMIN_HOST": {
			newKey: "DAGU_HOST",
			setter: func(c *Config, v string) { c.Host = v },
		},
		"DAGU__DATA": {
			newKey: "DAGU_DATA_DIR",
			setter: func(c *Config, v string) { c.Paths.DataDir = v },
		},
		"DAGU__SUSPEND_FLAGS_DIR": {
			newKey: "DAGU_SUSPEND_FLAGS_DIR",
			setter: func(c *Config, v string) { c.Paths.SuspendFlagsDir = v },
		},
		"DAGU__ADMIN_LOGS_DIR": {
			newKey: "DAGU_ADMIN_LOG_DIR",
			setter: func(c *Config, v string) { c.Paths.AdminLogsDir = v },
		},
	}

	for oldKey, mapping := range legacyEnvs {
		if value := os.Getenv(oldKey); value != "" {
			log.Printf("%s is deprecated. Use %s instead.", oldKey, mapping.newKey)
			mapping.setter(cfg, value)
		}
	}

	return nil
}

func (l *ConfigLoader) setTimezone(cfg *Config) error {
	if cfg.TZ != "" {
		loc, err := time.LoadLocation(cfg.TZ)
		if err != nil {
			return fmt.Errorf("failed to load timezone: %w", err)
		}
		cfg.Location = loc
		os.Setenv("TZ", cfg.TZ)
	} else {
		_, offset := time.Now().Zone()
		if offset == 0 {
			cfg.TZ = "UTC"
		} else {
			cfg.TZ = fmt.Sprintf("UTC%+d", offset/3600)
		}
		cfg.Location = time.Local
	}
	return nil
}

func (l *ConfigLoader) setExecutable(cfg *Config) error {
	if cfg.Paths.Executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		cfg.Paths.Executable = executable
	}
	return nil
}

func (l *ConfigLoader) setExecutableDefault() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	viper.SetDefault("executable", executable)
	return nil
}

func (l *ConfigLoader) validateConfig(cfg *Config) error {
	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Port)
	}

	if cfg.Auth.Basic.Enabled && (cfg.Auth.Basic.Username == "" || cfg.Auth.Basic.Password == "") {
		return fmt.Errorf("basic auth enabled but username or password is not set")
	}

	if cfg.Auth.Token.Enabled && cfg.Auth.Token.Value == "" {
		return fmt.Errorf("auth token enabled but token is not set")
	}

	if cfg.TLS != nil {
		if cfg.TLS.CertFile == "" || cfg.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}

	if cfg.Port < 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Port)
	}

	if cfg.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", cfg.UI.MaxDashboardPageLimit)
	}

	return nil
}
