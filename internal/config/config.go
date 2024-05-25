package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Host               string
	Port               int
	DAGs               string
	Executable         string
	WorkDir            string
	IsBasicAuth        bool
	BasicAuthUsername  string
	BasicAuthPassword  string
	LogEncodingCharset string
	LogDir             string
	DataDir            string
	SuspendFlagsDir    string
	AdminLogsDir       string
	BaseConfig         string
	NavbarColor        string
	NavbarTitle        string
	Env                sync.Map
	TLS                *TLS
	IsAuthToken        bool
	AuthToken          string
	LatestStatusToday  bool
}

func (cfg *Config) GetAPIBaseURL() string {
	return "/api/v1"
}

type TLS struct {
	CertFile string
	KeyFile  string
}

var (
	cache = &configCache{}
)

type configCache struct {
	instance *Config
	mu       sync.RWMutex
}

func (cc *configCache) getConfig() *Config {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.instance
}

func (cc *configCache) setConfig(cfg *Config) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.instance = cfg
}

func Get() *Config {
	cfg := cache.getConfig()
	if cfg != nil {
		return cfg
	}
	if err := LoadConfig(); err != nil {
		panic(err)
	}
	return cache.getConfig()
}

func LoadConfig() error {
	appHome := appHomeDir()

	viper.SetEnvPrefix("dagu")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

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

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", "8080")
	viper.SetDefault("executable", executable)
	viper.SetDefault("dags", path.Join(appHome, "dags"))
	viper.SetDefault("workDir", "")
	viper.SetDefault("isBasicAuth", "0")
	viper.SetDefault("basicAuthUsername", "")
	viper.SetDefault("basicAuthPassword", "")
	viper.SetDefault("logEncodingCharset", "")
	viper.SetDefault("baseConfig", path.Join(appHome, "config.yaml"))
	viper.SetDefault("logDir", path.Join(appHome, "logs"))
	viper.SetDefault("dataDir", path.Join(appHome, "data"))
	viper.SetDefault("suspendFlagsDir", path.Join(appHome, "suspend"))
	viper.SetDefault("adminLogsDir", path.Join(appHome, "logs", "admin"))
	viper.SetDefault("navbarColor", "")
	viper.SetDefault("navbarTitle", "Dagu")
	viper.SetDefault("isAuthToken", "0")
	viper.SetDefault("authToken", "0")
	viper.SetDefault("latestStatusToday", "0")

	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal cfg file: %w", err)
	}
	loadLegacyEnvs(cfg)
	loadEnvs(cfg)

	cache.setConfig(cfg)

	return nil
}

func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(err)
	}
	return home
}

func loadEnvs(cfg *Config) {
	cfg.Env.Range(func(k, v interface{}) bool {
		_ = os.Setenv(k.(string), v.(string))
		return true
	})
}

func loadLegacyEnvs(cfg *Config) {
	// For backward compatibility.
	cfg.NavbarColor = getEnv("DAGU__ADMIN_NAVBAR_COLOR", cfg.NavbarColor)
	cfg.NavbarTitle = getEnv("DAGU__ADMIN_NAVBAR_TITLE", cfg.NavbarTitle)
	cfg.Port = getEnvI("DAGU__ADMIN_PORT", cfg.Port)
	cfg.Host = getEnv("DAGU__ADMIN_HOST", cfg.Host)
	cfg.DataDir = getEnv("DAGU__DATA", cfg.DataDir)
	cfg.LogDir = getEnv("DAGU__DATA", cfg.LogDir)
	cfg.SuspendFlagsDir = getEnv("DAGU__SUSPEND_FLAGS_DIR", cfg.SuspendFlagsDir)
	cfg.BaseConfig = getEnv("DAGU__SUSPEND_FLAGS_DIR", cfg.BaseConfig)
	cfg.AdminLogsDir = getEnv("DAGU__ADMIN_LOGS_DIR", cfg.AdminLogsDir)
}

func getEnv(env, def string) string {
	v := os.Getenv(env)
	if v == "" {
		return def
	}
	return v
}

func parseInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

func getEnvI(env string, def int) int {
	v := os.Getenv(env)
	if v == "" {
		return def
	}
	return parseInt(v)
}

const (
	appHomeEnv     = "DAGU_HOME"
	appHomeDefault = ".dagu"
)

func appHomeDir() string {
	appDir := os.Getenv(appHomeEnv)
	if appDir == "" {
		return path.Join(homeDir(), appHomeDefault)
	}
	return appDir
}
