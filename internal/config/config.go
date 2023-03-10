package config

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/spf13/viper"
)

type Config struct {
	Host               string `mapstructure:"host"`
	Port               string `mapstructure:"port"`
	DAGs               string `mapstructure:"dags_dir"`
	Command            string `mapstructure:"command"`
	WorkDir            string `mapstructure:"work_dir"`
	IsBasicAuth        bool   `mapstructure:"is_basicauth"`
	BasicAuthUsername  string `mapstructure:"basicauth_username"`
	BasicAuthPassword  string `mapstructure:"basicauth_password"`
	LogEncodingCharset string `mapstructure:"log_encoding_charset"`
	LogDir             string `mapstructure:"log_dir"`
	DataDir            string `mapstructure:"data_dir"`
	SuspendFlagsDir    string `mapstructure:"suspend_flags_dir"`
	AdminLogsDir       string `mapstructure:"admin_log_dir"`
	BaseConfig         string `mapstructure:"base_config"`
	NavbarColor        string `mapstructure:"navbar_color"`
	NavbarTitle        string `mapstructure:"navbar_title"`
	Env                []string
}

var C *Config = nil

func LoadConfig(homeDir string) error {
	appHome := path.Join(homeDir, appHomeDir())

	log.Printf("Config file used: [%s]", viper.ConfigFileUsed())

	viper.AutomaticEnv()
	viper.SetEnvPrefix("dagu")

	viper.SetDefault("host", "127.0.0.1")
	viper.SetDefault("port", "8080")
	viper.SetDefault("dags_dir", path.Join(appHome, "dags"))
	viper.SetDefault("command", "dagu")
	viper.SetDefault("work_dir", "")
	viper.SetDefault("is_basicauth", "0")
	viper.SetDefault("basicauth_username", "")
	viper.SetDefault("basicauth_password", "")
	viper.SetDefault("log_encoding_charset", "")
	viper.SetDefault("base_config", path.Join(appHome, "config.yaml"))
	viper.SetDefault("log_dir", path.Join(appHome, "logs"))
	viper.SetDefault("data_dir", path.Join(appHome, "data"))
	viper.SetDefault("suspend_flags_dir", path.Join(appHome, "suspend"))
	viper.SetDefault("admin_log_dir", path.Join(appHome, "logs", "admin"))
	viper.SetDefault("navbar_color", "")
	viper.SetDefault("navbar_title", "Dagu")

	if err := viper.ReadInConfig(); err == nil {
		log.Printf("Config file used: [%s]", viper.ConfigFileUsed())
	}

	cfg := &Config{}
	err := viper.Unmarshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Config file: %w", err)
	}
	C = cfg
	loadLegacyEnvs()

	return nil
}

func loadLegacyEnvs() {
	// For backward compatibility.
	C.NavbarColor = loadEnv("DAGU__ADMIN_NAVBAR_COLOR", C.NavbarColor)
	C.NavbarTitle = loadEnv("DAGU__ADMIN_NAVBAR_TITLE", C.NavbarTitle)
	C.Port = loadEnv("DAGU__ADMIN_PORT", C.Port)
	C.Host = loadEnv("DAGU__ADMIN_HOST", C.Host)
	C.DataDir = loadEnv("DAGU__DATA", C.DataDir)
	C.LogDir = loadEnv("DAGU__DATA", C.LogDir)
	C.SuspendFlagsDir = loadEnv("DAGU__SUSPEND_FLAGS_DIR", C.SuspendFlagsDir)
	C.BaseConfig = loadEnv("DAGU__SUSPEND_FLAGS_DIR", C.BaseConfig)
	C.AdminLogsDir = loadEnv("DAGU__ADMIN_LOGS_DIR", C.AdminLogsDir)
}

func loadEnv(env, def string) string {
	v := os.Getenv("env")
	if v == "" {
		return def
	}
	return v
}

const (
	appHomeEnv     = "DAGU_HOME"
	appHomeDefault = ".dagu"
)

func appHomeDir() string {
	appDir := os.Getenv(appHomeEnv)
	if appDir == "" {
		return appHomeDefault
	}
	return appDir
}
