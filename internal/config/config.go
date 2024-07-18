package config

import (
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/adrg/xdg"
	"github.com/spf13/viper"
)

// Config represents the configuration for the server.
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

type TLS struct {
	CertFile string
	KeyFile  string
}

var configLock sync.Mutex

const envPrefix = "DAGU"

func Load() (*Config, error) {
	configLock.Lock()
	defer configLock.Unlock()

	viper.SetEnvPrefix(envPrefix)
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Set default values for config keys.
	if err := setupViper(); err != nil {
		return nil, err
	}

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
		if err := os.Setenv(k.(string), v.(string)); err != nil {
			log.Printf("failed to set env variable %s: %v", k, err)
		}
		return true
	})

	return &cfg, nil
}

const (
	// Application name.
	appName = "dagu"
)

func setupViper() error {
	// Bind environment variables with config keys.
	bindEnvs()

	// Set default values for config keys.

	// Directories
	baseDirs := getBaseDirs()
	viper.SetDefault("dags", baseDirs.dags)
	viper.SetDefault("suspendFlagsDir", baseDirs.suspendFlags)
	viper.SetDefault("dataDir", baseDirs.data)
	viper.SetDefault("logDir", baseDirs.logs)
	viper.SetDefault("adminLogsDir", baseDirs.adminLogs)

	// Base config file
	viper.SetDefault("baseConfig", getBaseConfigPath(baseDirs))

	// Other defaults
	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", "8080")
	viper.SetDefault("navbarTitle", "Dagu")
	viper.SetDefault("apiBaseURL", "/api/v1")

	// Set executable path
	// This is used for invoking the workflow process on the server.
	return setExecutableDefault()
}

type baseDirs struct {
	config       string
	dags         string
	suspendFlags string
	data         string
	logs         string
	adminLogs    string
}

const (
	// Constants for config.
	legacyConfigDir       = ".dagu"
	legacyConfigDirEnvKey = "DAGU_HOME"

	// default directories
	dagsDir    = "dags"
	suspendDir = "suspend"
)

var (
	// Config directories
	ConfigDir = getConfigDir()
)

func getBaseDirs() baseDirs {
	logsDir := getLogsDir()
	return baseDirs{
		config:       ConfigDir,
		dags:         path.Join(ConfigDir, dagsDir),
		suspendFlags: path.Join(ConfigDir, suspendDir),
		data:         getDataDir(),
		logs:         logsDir,
		adminLogs:    path.Join(logsDir, "admin"),
	}
}

func setExecutableDefault() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	viper.SetDefault("executable", executable)
	return nil
}

func getLogsDir() string {
	if v, ok := getLegacyConfigPath(); ok {
		// For backward compatibility.
		return filepath.Join(v, "logs")
	}
	return filepath.Join(xdg.DataHome, appName, "logs")
}

func getDataDir() string {
	if v, ok := getLegacyConfigPath(); ok {
		// For backward compatibility.
		return filepath.Join(v, "data")
	}
	return filepath.Join(xdg.DataHome, appName, "history")
}

func getConfigDir() string {
	if v, ok := getLegacyConfigPath(); ok {
		return v
	}
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return filepath.Join(v, appName)
	}
	return filepath.Join(getHomeDir(), ".config", appName)
}

func getHomeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("could not determine home directory: %v", err)
		return ""
	}
	return dir
}

const (
	// Base config file name for all DAGs.
	baseConfig = "base.yaml"
	// Legacy config path for backward compatibility.
	legacyBaseConfig = "config.yaml"
)

func getBaseConfigPath(b baseDirs) string {
	legacyPath := filepath.Join(b.config, legacyBaseConfig)
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath
	}
	return filepath.Join(b.config, baseConfig)
}

func getLegacyConfigPath() (string, bool) {
	// For backward compatibility.
	// If the environment variable is set, use it.
	if v := os.Getenv(legacyConfigDirEnvKey); v != "" {
		return v, true
	}
	// If not, check if the legacyPath config directory exists.
	legacyPath := filepath.Join(getHomeDir(), legacyConfigDir)
	if _, err := os.Stat(legacyPath); err == nil {
		return legacyPath, true
	}
	return "", false
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
	// Load old environment variables if they exist.
	if v := os.Getenv("DAGU__ADMIN_NAVBAR_COLOR"); v != "" {
		log.Println("DAGU__ADMIN_NAVBAR_COLOR is deprecated. Use DAGU_NAVBAR_COLOR instead.")
		cfg.NavbarColor = v
	}
	if v := os.Getenv("DAGU__ADMIN_NAVBAR_TITLE"); v != "" {
		log.Println("DAGU__ADMIN_NAVBAR_TITLE is deprecated. Use DAGU_NAVBAR_TITLE instead.")
		cfg.NavbarTitle = v
	}
	if v := os.Getenv("DAGU__ADMIN_PORT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			log.Println("DAGU__ADMIN_PORT is deprecated. Use DAGU_PORT instead.")
			cfg.Port = i
		}
	}
	if v := os.Getenv("DAGU__ADMIN_HOST"); v != "" {
		log.Println("DAGU__ADMIN_HOST is deprecated. Use DAGU_HOST instead.")
		cfg.Host = v
	}
	if v := os.Getenv("DAGU__DATA"); v != "" {
		log.Println("DAGU__DATA is deprecated. Use DAGU_DATA_DIR instead.")
		cfg.DataDir = v
	}
	if v := os.Getenv("DAGU__SUSPEND_FLAGS_DIR"); v != "" {
		log.Println("DAGU__SUSPEND_FLAGS_DIR is deprecated. Use DAGU_SUSPEND_FLAGS_DIR instead.")
		cfg.SuspendFlagsDir = v
	}
	if v := os.Getenv("DAGU__ADMIN_LOGS_DIR"); v != "" {
		log.Println("DAGU__ADMIN_LOGS_DIR is deprecated. Use DAGU_ADMIN_LOG_DIR instead.")
		cfg.AdminLogsDir = v
	}
}
