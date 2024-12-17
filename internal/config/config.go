// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
	"time"

	"github.com/adrg/xdg"
	"github.com/dagu-org/dagu/internal/build"
	"github.com/spf13/viper"
)

// Config represents the configuration for the server.
type Config struct {
	RemoteNodes           []RemoteNode   // Remote node API URLs (e.g., http://localhost:8080/api/v1)
	Host                  string         // Server host
	Port                  int            // Server port
	DAGs                  string         // Location of DAG files
	Executable            string         // Executable path
	WorkDir               string         // Default working directory
	IsBasicAuth           bool           // Enable basic auth
	BasicAuthUsername     string         // Basic auth username
	BasicAuthPassword     string         // Basic auth password
	LogEncodingCharset    string         // Log encoding charset
	LogDir                string         // Log directory
	DataDir               string         // Data directory
	SuspendFlagsDir       string         // Suspend flags directory
	AdminLogsDir          string         // Directory for admin logs
	BaseConfig            string         // Common config file for all DAGs.
	NavbarColor           string         // Navbar color for the web UI
	NavbarTitle           string         // Navbar title for the web UI
	Env                   sync.Map       // Store environment variables
	TLS                   *TLS           // TLS configuration
	IsAuthToken           bool           // Enable auth token for API
	AuthToken             string         // Auth token for API
	LatestStatusToday     bool           // Show latest status today or the latest status
	BasePath              string         // Base path for the server
	APIBaseURL            string         // Base URL for API
	Debug                 bool           // Enable debug mode (verbose logging)
	LogFormat             string         // Log format
	TZ                    string         // The server time zone
	Location              *time.Location // The server location
	MaxDashboardPageLimit int            // The default page limit for the dashboard
}

// RemoteNode is the configuration for a remote host that can be proxied by the server.
// This is useful for fetching data from a remote host and displaying it on the server.
type RemoteNode struct {
	Name              string // Name of the remote host
	APIBaseURL        string // Base URL for the remote host API (e.g., http://localhost:9090/api/v1)
	IsBasicAuth       bool   // Enable basic auth
	BasicAuthUsername string // Basic auth username
	BasicAuthPassword string // Basic auth password
	IsAuthToken       bool   // Enable auth token for API
	AuthToken         string // Auth token for API
	SkipTLSVerify     bool   // Skip TLS verification
}

type TLS struct {
	CertFile string
	KeyFile  string
}

var configLock sync.Mutex

func Load() (*Config, error) {
	configLock.Lock()
	defer configLock.Unlock()

	if err := setupViper(); err != nil {
		return nil, err
	}

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

	loadLegacyEnvs(&cfg)
	setEnvVariables(&cfg)
	setTimezone(&cfg)
	cleanBasePath(&cfg)

	return &cfg, nil
}

func setEnvVariables(cfg *Config) {
	cfg.Env.Range(func(k, v any) bool {
		if err := os.Setenv(k.(string), v.(string)); err != nil {
			log.Printf("failed to set env variable %s: %v", k, err)
		}
		return true
	})
}

func setTimezone(cfg *Config) error {
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

func cleanBasePath(cfg *Config) {
	if cfg.BasePath != "" {
		cfg.BasePath = path.Clean(cfg.BasePath)

		if !path.IsAbs(cfg.BasePath) {
			cfg.BasePath = path.Join("/", cfg.BasePath)
		}

		if cfg.BasePath == "/" {
			cfg.BasePath = ""
		}
	}
}

func setupViper() error {
	homeDir := getHomeDir()

	var xdgCfg XDGConfig
	xdgCfg.DataHome = xdg.DataHome
	xdgCfg.ConfigHome = filepath.Join(homeDir, ".config")
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		xdgCfg.ConfigHome = v
	}

	r := newResolver("DAGU_HOME", filepath.Join(homeDir, ".dagu"), xdgCfg)

	viper.AddConfigPath(r.configDir)
	viper.SetConfigType("yaml")
	viper.SetConfigName("admin")

	viper.SetEnvPrefix(strings.ToUpper(build.Slug))
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	// Bind environment variables with config keys.
	bindEnvs()

	// Set default values for config keys.
	viper.SetDefault("dags", r.dagsDir)
	viper.SetDefault("dagsDir", r.dagsDir) // For backward compatibility
	viper.SetDefault("suspendFlagsDir", r.suspendFlagsDir)
	viper.SetDefault("dataDir", r.dataDir)
	viper.SetDefault("logDir", r.logsDir)
	viper.SetDefault("adminLogsDir", r.adminLogsDir)
	viper.SetDefault("baseConfig", r.baseConfigFile)

	// Logging configurations
	viper.SetDefault("logLevel", "info")
	viper.SetDefault("logFormat", "text")

	// Other defaults
	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", "8080")
	viper.SetDefault("navbarTitle", build.AppName)
	viper.SetDefault("basePath", "")
	viper.SetDefault("apiBaseURL", "/api/v1")
	viper.SetDefault("maxDashboardPageLimit", 100)

	// Set executable path
	// This is used for invoking the workflow process on the server.
	return setExecutableDefault()
}

func getHomeDir() string {
	dir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("could not determine home directory: %v", err)
		return ""
	}
	return dir
}

func setExecutableDefault() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	viper.SetDefault("executable", executable)
	return nil
}

func bindEnvs() {
	// Server configurations
	_ = viper.BindEnv("logEncodingCharset", "DAGU_LOG_ENCODING_CHARSET")
	_ = viper.BindEnv("navbarColor", "DAGU_NAVBAR_COLOR")
	_ = viper.BindEnv("navbarTitle", "DAGU_NAVBAR_TITLE")
	_ = viper.BindEnv("basePath", "DAGU_BASE_PATH")
	_ = viper.BindEnv("apiBaseURL", "DAGU_API_BASE_URL")
	_ = viper.BindEnv("tz", "DAGU_TZ")
	_ = viper.BindEnv("maxDashboardPageLimit", "DAGU_MAX_DASHBOARD_PAGE_LIMIT")

	// Basic authentication
	_ = viper.BindEnv("isBasicAuth", "DAGU_IS_BASICAUTH")
	_ = viper.BindEnv("basicAuthUsername", "DAGU_BASICAUTH_USERNAME")
	_ = viper.BindEnv("basicAuthPassword", "DAGU_BASICAUTH_PASSWORD")

	// TLS configurations
	_ = viper.BindEnv("tls.certFile", "DAGU_CERT_FILE")
	_ = viper.BindEnv("tls.keyFile", "DAGU_KEY_FILE")

	// Auth Token
	_ = viper.BindEnv("isAuthToken", "DAGU_IS_AUTHTOKEN")
	_ = viper.BindEnv("authToken", "DAGU_AUTHTOKEN")

	// Executables
	_ = viper.BindEnv("executable", "DAGU_EXECUTABLE")

	// Directories and files
	_ = viper.BindEnv("dags", "DAGU_DAGS_DIR")
	_ = viper.BindEnv("workDir", "DAGU_WORK_DIR")
	_ = viper.BindEnv("baseConfig", "DAGU_BASE_CONFIG")
	_ = viper.BindEnv("logDir", "DAGU_LOG_DIR")
	_ = viper.BindEnv("dataDir", "DAGU_DATA_DIR")
	_ = viper.BindEnv("suspendFlagsDir", "DAGU_SUSPEND_FLAGS_DIR")
	_ = viper.BindEnv("adminLogsDir", "DAGU_ADMIN_LOG_DIR")

	// Miscellaneous
	_ = viper.BindEnv("latestStatusToday", "DAGU_LATEST_STATUS")
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
