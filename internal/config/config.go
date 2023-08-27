package config

import (
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/utils"
)

type Config struct {
	Host               string
	Port               int
	DAGs               string
	Command            string
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
	Env                map[string]string
	TLS                *struct {
		CertFile string
		KeyFile  string
	}
}

var instance *Config = nil

func Get() *Config {
	if instance == nil {
		home, _ := os.UserHomeDir()
		if err := LoadConfig(home); err != nil {
			panic(err)
		}
	}
	return instance
}

func LoadConfig(userHomeDir string) error {
	appHome := appHomeDir(userHomeDir)

	viper.SetEnvPrefix("dagu")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))

	viper.BindEnv("command", "DAGU_EXECUTABLE")
	viper.BindEnv("dags", "DAGU_DAGS_DIR")
	viper.BindEnv("workDir", "DAGU_WORK_DIR")
	viper.BindEnv("isBasicAuth", "DAGU_IS_BASICAUTH")
	viper.BindEnv("basicAuthUsername", "DAGU_BASICAUTH_USERNAME")
	viper.BindEnv("basicAuthPassword", "DAGU_BASICAUTH_PASSWORD")
	viper.BindEnv("logEncodingCharset", "DAGU_LOG_ENCODING_CHARSET")
	viper.BindEnv("baseConfig", "DAGU_BASE_CONFIG")
	viper.BindEnv("logDir", "DAGU_LOG_DIR")
	viper.BindEnv("dataDir", "DAGU_DATA_DIR")
	viper.BindEnv("suspendFlagsDir", "DAGU_SUSPEND_FLAGS_DIR")
	viper.BindEnv("adminLogsDir", "DAGU_ADMIN_LOG_DIR")
	viper.BindEnv("navbarColor", "DAGU_NAVBAR_COLOR")
	viper.BindEnv("navbarTitle", "DAGU_NAVBAR_TITLE")
	viper.BindEnv("tls.certFile", "DAGU_CERT_FILE")
	viper.BindEnv("tls.keyFile", "DAGU_KEY_FILE")

	exectable := "dagu"
	if ex, err := os.Executable(); err == nil {
		exectable = ex
	}

	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", "8080")
	viper.SetDefault("command", exectable)
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

	viper.AutomaticEnv()

	_ = viper.ReadInConfig()

	cfg := &Config{}
	err := viper.Unmarshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal cfg file: %w", err)
	}
	instance = cfg
	loadLegacyEnvs()
	loadEnvs()

	return nil
}

func loadEnvs() {
	if instance.Env == nil {
		instance.Env = map[string]string{}
	}
	for k, v := range instance.Env {
		_ = os.Setenv(k, v)
	}
	for k, v := range utils.DefaultEnv() {
		if _, ok := instance.Env[k]; !ok {
			instance.Env[k] = v
		}
	}
}

func loadLegacyEnvs() {
	// For backward compatibility.
	instance.NavbarColor = getEnv("DAGU__ADMIN_NAVBAR_COLOR", instance.NavbarColor)
	instance.NavbarTitle = getEnv("DAGU__ADMIN_NAVBAR_TITLE", instance.NavbarTitle)
	instance.Port = getEnvI("DAGU__ADMIN_PORT", instance.Port)
	instance.Host = getEnv("DAGU__ADMIN_HOST", instance.Host)
	instance.DataDir = getEnv("DAGU__DATA", instance.DataDir)
	instance.LogDir = getEnv("DAGU__DATA", instance.LogDir)
	instance.SuspendFlagsDir = getEnv("DAGU__SUSPEND_FLAGS_DIR", instance.SuspendFlagsDir)
	instance.BaseConfig = getEnv("DAGU__SUSPEND_FLAGS_DIR", instance.BaseConfig)
	instance.AdminLogsDir = getEnv("DAGU__ADMIN_LOGS_DIR", instance.AdminLogsDir)
}

func getEnv(env, def string) string {
	v := os.Getenv("env")
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
	v := os.Getenv("env")
	if v == "" {
		return def
	}
	return parseInt(v)
}

const (
	appHomeEnv     = "DAGU_HOME"
	appHomeDefault = ".dagu"
)

func appHomeDir(userHomeDir string) string {
	appDir := os.Getenv(appHomeEnv)
	if appDir == "" {
		return path.Join(userHomeDir, appHomeDefault)
	}
	return appDir
}
