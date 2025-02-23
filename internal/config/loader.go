package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/viper"
)

// Load creates a new configuration.
func Load(opts ...ConfigLoaderOption) (*Config, error) {
	loader := NewConfigLoader(opts...)
	cfg, err := loader.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

type ConfigLoader struct {
	lock       sync.Mutex
	configFile string
	warnings   []string
}

type ConfigLoaderOption func(*ConfigLoader)

func WithConfigFile(configFile string) ConfigLoaderOption {
	return func(l *ConfigLoader) {
		l.configFile = configFile
	}
}

func NewConfigLoader(options ...ConfigLoaderOption) *ConfigLoader {
	loader := &ConfigLoader{}
	for _, option := range options {
		option(loader)
	}
	return loader
}

func (l *ConfigLoader) Load() (*Config, error) {
	l.lock.Lock()
	defer l.lock.Unlock()

	if err := l.setupViper(); err != nil {
		return nil, fmt.Errorf("viper setup failed: %w", err)
	}

	var def Definition
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

	if err := viper.Unmarshal(&def); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfg, err := l.buildConfig(def)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	cfg.Warnings = l.warnings

	return cfg, nil
}

func (l *ConfigLoader) buildConfig(def Definition) (*Config, error) {
	var cfg Config

	cfg.Global = Global{
		Debug:     def.Debug,
		LogFormat: def.LogFormat,
		TZ:        def.TZ,
		WorkDir:   def.WorkDir,
	}

	if err := cfg.Global.setTimezone(); err != nil {
		return nil, fmt.Errorf("failed to set timezone: %w", err)
	}

	cfg.Server = Server{
		Host:        def.Host,
		Port:        def.Port,
		BasePath:    def.BasePath,
		APIBasePath: def.APIBaseURL,
	}

	for _, node := range def.RemoteNodes {
		cfg.Server.RemoteNodes = append(cfg.Server.RemoteNodes, RemoteNode{
			Name:       node.Name,
			APIBaseURL: node.APIBaseURL,
		})
	}

	if def.APIBaseURL != "" {
		cfg.Server.APIBasePath = def.APIBaseURL
	}

	if def.Headless != nil {
		cfg.Server.Headless = *def.Headless
	}

	if def.LatestStatusToday != nil {
		cfg.Server.LatestStatusToday = *def.LatestStatusToday
	}

	if def.TLS != nil {
		cfg.Server.TLS = &TLSConfig{
			CertFile: def.TLS.CertFile,
			KeyFile:  def.TLS.KeyFile,
		}
	}

	if def.Auth != nil {
		if def.Auth.Basic != nil {
			cfg.Server.Auth.Basic.Enabled = def.Auth.Basic.Enabled
			cfg.Server.Auth.Basic.Username = def.Auth.Basic.Username
			cfg.Server.Auth.Basic.Password = def.Auth.Basic.Password
		}
		if def.Auth.Token != nil {
			cfg.Server.Auth.Token.Enabled = def.Auth.Token.Enabled
			cfg.Server.Auth.Token.Value = def.Auth.Token.Value
		}
	}

	cfg.Server.cleanBasePath()

	if def.Paths != nil {
		cfg.Paths.DAGsDir = def.Paths.DAGsDir
		cfg.Paths.SuspendFlagsDir = def.Paths.SuspendFlagsDir
		cfg.Paths.DataDir = def.Paths.DataDir
		cfg.Paths.LogDir = def.Paths.LogDir
		cfg.Paths.AdminLogsDir = def.Paths.AdminLogsDir
		cfg.Paths.BaseConfig = def.Paths.BaseConfig
		cfg.Paths.Executable = def.Paths.Executable
	}

	if def.UI != nil {
		cfg.UI.NavbarColor = def.UI.NavbarColor
		cfg.UI.NavbarTitle = def.UI.NavbarTitle
		cfg.UI.MaxDashboardPageLimit = def.UI.MaxDashboardPageLimit
		cfg.UI.LogEncodingCharset = def.UI.LogEncodingCharset
	}

	l.LoadLegacyFields(&cfg, def)
	l.LoadLegacyEnv(&cfg)

	if err := l.setExecutable(&cfg); err != nil {
		return nil, fmt.Errorf("failed to set executable: %w", err)
	}
	if err := l.validateConfig(&cfg); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

func (l *ConfigLoader) LoadLegacyFields(cfg *Config, def Definition) {
	if def.BasicAuthUsername != "" {
		cfg.Server.Auth.Basic.Username = def.BasicAuthUsername
	}
	if def.BasicAuthPassword != "" {
		cfg.Server.Auth.Basic.Password = def.BasicAuthPassword
	}
	if def.IsAuthToken {
		cfg.Server.Auth.Token.Enabled = true
		cfg.Server.Auth.Token.Value = def.AuthToken
	}
	if def.DAGs != "" {
		cfg.Paths.DAGsDir = def.DAGs
	}
	if def.DAGsDir != "" {
		cfg.Paths.DAGsDir = def.DAGsDir
	}
	if def.Executable != "" {
		cfg.Paths.Executable = def.Executable
	}
	if def.LogDir != "" {
		cfg.Paths.LogDir = def.LogDir
	}
	if def.DataDir != "" {
		cfg.Paths.DataDir = def.DataDir
	}
	if def.SuspendFlagsDir != "" {
		cfg.Paths.SuspendFlagsDir = def.SuspendFlagsDir
	}
	if def.AdminLogsDir != "" {
		cfg.Paths.AdminLogsDir = def.AdminLogsDir
	}
	if def.BaseConfig != "" {
		cfg.Paths.BaseConfig = def.BaseConfig
	}
	if def.LogEncodingCharset != "" {
		cfg.UI.LogEncodingCharset = def.LogEncodingCharset
	}
	if def.NavbarColor != "" {
		cfg.UI.NavbarColor = def.NavbarColor
	}
	if def.NavbarTitle != "" {
		cfg.UI.NavbarTitle = def.NavbarTitle
	}
	if def.MaxDashboardPageLimit > 0 {
		cfg.UI.MaxDashboardPageLimit = def.MaxDashboardPageLimit
	}
}

func (l *ConfigLoader) setupViper() error {
	homeDir, err := l.getHomeDir()
	if err != nil {
		return err
	}
	xdgConfig := l.getXDGConfig(homeDir)
	resolver := NewResolver("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgConfig)

	l.warnings = append(l.warnings, resolver.Warnings...)

	l.configureViper(resolver)
	l.bindEnvironmentVariables()
	l.setDefaultValues(resolver)

	return nil
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
	if l.configFile == "" {
		viper.AddConfigPath(resolver.ConfigDir)
		viper.SetConfigName("config")
	} else {
		viper.SetConfigFile(l.configFile)
	}
	viper.SetConfigType("yaml")
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
	viper.SetDefault("apiBasePath", "/api/v1")
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
	l.bindEnv("headless", "HEADLESS")

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

func (l *ConfigLoader) LoadLegacyEnv(cfg *Config) {
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
					c.Server.Port = i
				}
			},
		},
		"DAGU__ADMIN_HOST": {
			newKey: "DAGU_HOST",
			setter: func(c *Config, v string) { c.Server.Host = v },
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

func (l *ConfigLoader) validateConfig(cfg *Config) error {
	if cfg.Server.Auth.Basic.Enabled && (cfg.Server.Auth.Basic.Username == "" || cfg.Server.Auth.Basic.Password == "") {
		return fmt.Errorf("basic auth enabled but username or password is not set")
	}

	if cfg.Server.Auth.Token.Enabled && cfg.Server.Auth.Token.Value == "" {
		return fmt.Errorf("auth token enabled but token is not set")
	}

	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}

	if cfg.Server.TLS != nil {
		if cfg.Server.TLS.CertFile == "" || cfg.Server.TLS.KeyFile == "" {
			return fmt.Errorf("TLS configuration incomplete: both cert and key files are required")
		}
	}

	if cfg.Server.Port < 0 || cfg.Server.Port > 65535 {
		return fmt.Errorf("invalid port number: %d", cfg.Server.Port)
	}

	if cfg.UI.MaxDashboardPageLimit < 1 {
		return fmt.Errorf("invalid max dashboard page limit: %d", cfg.UI.MaxDashboardPageLimit)
	}

	return nil
}
