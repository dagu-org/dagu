package config

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	Host               string   // Server host
	Port               int      // Server port
	DAGs               string   // Location of DAG files
	Executable         string   // Executable path
	WorkDir            string   // Default working directory
	IsBasicAuth        bool     // Enable basic auth
	BasicAuthUsername  string   // Basic auth username
	BasicAuthPassword  string   // Basic auth password
	LogEncodingCharset string   // Log encoding charset
	LogDir             string   // Log directory
	DataDir            string   // Data directory
	SuspendFlagsDir    string   // Suspend flags directory
	AdminLogsDir       string   // Directory for admin logs
	BaseConfig         string   // Common config file for all DAGs.
	NavbarColor        string   // Navbar color for the web UI
	NavbarTitle        string   // Navbar title for the web UI
	Env                sync.Map // Store environment variables
	TLS                *TLS     // TLS configuration
	IsAuthToken        bool     // Enable auth token for API
	AuthToken          string   // Auth token for API
	LatestStatusToday  bool     // Show latest status today or the latest status
	APIBaseURL         string   // Base URL for API
}

const (
	// Constants for config.
	envPrefix      = "DAGU"
	appHomeDefault = ".dagu"
	legacyAppHome  = "DAGU_HOME"

	// Default base config file.
	baseConfig = "config.yaml"

	// default directories
	dagsDir    = "dags"
	dataDir    = "data"
	logDir     = "logs"
	suspendDir = "suspend"
)

var adminLogsDir = path.Join(logDir, "admin")

var (
	defaults = Config{
		Host:              "127.0.0.1",
		Port:              8080,
		IsBasicAuth:       false,
		NavbarTitle:       "Dagu",
		IsAuthToken:       false,
		LatestStatusToday: false,
		APIBaseURL:        "/api/v1",
	}
)

type TLS struct {
	CertFile string
	KeyFile  string
}

var (
	lock sync.Mutex
)

func Load() (*Config, error) {
	lock.Lock()
	defer lock.Unlock()

	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind environment variables with config keys.
	bindEnvs()

	// Set default values for config keys.
	setDefaults()

	// Populate viper with environment variables.
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read cfg file: %w", err)
		}
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cfg file: %w", err)
	}

	// Load legacy environment variables if they exist.
	loadLegacyEnvs(&cfg)

	// Set environment variables specified in the config file.
	cfg.Env.Range(func(k, v any) bool {
		_ = os.Setenv(k.(string), v.(string))
		return true
	})

	return &cfg, nil
}

func setDefaults() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	paths := getDefaultPaths()

	viper.SetDefault("host", defaults.Host)
	viper.SetDefault("port", defaults.Port)
	viper.SetDefault("executable", executable)
	viper.SetDefault("dags", path.Join(paths.configDir, dagsDir))
	viper.SetDefault("workDir", defaults.WorkDir)
	viper.SetDefault("isBasicAuth", defaults.IsBasicAuth)
	viper.SetDefault("basicAuthUsername", defaults.BasicAuthUsername)
	viper.SetDefault("basicAuthPassword", defaults.BasicAuthPassword)
	viper.SetDefault("logEncodingCharset", defaults.LogEncodingCharset)
	viper.SetDefault("baseConfig", path.Join(paths.configDir, baseConfig))
	viper.SetDefault("logDir", path.Join(paths.configDir, logDir))
	viper.SetDefault("dataDir", path.Join(paths.configDir, dataDir))
	viper.SetDefault("suspendFlagsDir", path.Join(paths.configDir, suspendDir))
	viper.SetDefault("adminLogsDir", path.Join(paths.configDir, adminLogsDir))
	viper.SetDefault("navbarColor", defaults.NavbarColor)
	viper.SetDefault("navbarTitle", defaults.NavbarTitle)
	viper.SetDefault("isAuthToken", defaults.IsAuthToken)
	viper.SetDefault("authToken", defaults.AuthToken)
	viper.SetDefault("latestStatusToday", defaults.LatestStatusToday)
	viper.SetDefault("apiBaseURL", defaults.APIBaseURL)

	return nil
}

func bindEnvs() {
	_ = viper.BindEnv("executable", "DAGU_EXECUTABLE")
	_ = viper.BindEnv("dags", "DAGU_DAGS_DIR")
	_ = viper.BindEnv("workDir", "DAGU_WORK_DIR")
	_ = viper.BindEnv("isBasicAuth", "DAGU_IS_BASICAUTH")
	_ = viper.BindEnv("basicAuthUsername", "DAGU_BASICAUTH_USERNAME")
	_ = viper.BindEnv("basicAuthPassword", "DAGU_BASICAUTH_PASSWORD")
	_ = viper.BindEnv("logEncodingCharset", "DAGU_LOG_ENCODING_CHARSET")
	_ = viper.BindEnv("baseConfig", "DAGU_BASE_CONFIG")
	_ = viper.BindEnv("logDir", "DAGU_LOG_DIR")
	_ = viper.BindEnv("dataDir", "DAGU_DATA_DIR")
	_ = viper.BindEnv("suspendFlagsDir", "DAGU_SUSPEND_FLAGS_DIR")
	_ = viper.BindEnv("adminLogsDir", "DAGU_ADMIN_LOG_DIR")
	_ = viper.BindEnv("navbarColor", "DAGU_NAVBAR_COLOR")
	_ = viper.BindEnv("navbarTitle", "DAGU_NAVBAR_TITLE")
	_ = viper.BindEnv("tls.certFile", "DAGU_CERT_FILE")
	_ = viper.BindEnv("tls.keyFile", "DAGU_KEY_FILE")
	_ = viper.BindEnv("isAuthToken", "DAGU_IS_AUTHTOKEN")
	_ = viper.BindEnv("authToken", "DAGU_AUTHTOKEN")
	_ = viper.BindEnv("latestStatusToday", "DAGU_LATEST_STATUS")
	_ = viper.BindEnv("apiBaseURL", "DAGU_API_BASE_URL")
}

func loadLegacyEnvs(cfg *Config) {
	// For backward compatibility.
	if v := os.Getenv("DAGU__ADMIN_NAVBAR_COLOR"); v != "" {
		cfg.NavbarColor = v
	}
	if v := os.Getenv("DAGU__ADMIN_NAVBAR_TITLE"); v != "" {
		cfg.NavbarTitle = v
	}
	if v := os.Getenv("DAGU__ADMIN_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.Port = i
		}
	}
	if v := os.Getenv("DAGU__ADMIN_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("DAGU__DATA"); v != "" {
		cfg.DataDir = v
	}
	if v := os.Getenv("DAGU__SUSPEND_FLAGS_DIR"); v != "" {
		cfg.SuspendFlagsDir = v
	}
	if v := os.Getenv("DAGU__ADMIN_LOGS_DIR"); v != "" {
		cfg.AdminLogsDir = v
	}
}

type defaultPaths struct {
	configDir string
}

func getDefaultPaths() defaultPaths {
	var paths defaultPaths

	if appDir := os.Getenv(legacyAppHome); appDir != "" {
		paths.configDir = appDir
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			panic(err)
		}
		paths.configDir = path.Join(home, appHomeDefault)
	}

	return paths
}
