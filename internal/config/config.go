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
	instance = cfg
	loadLegacyEnvs()

	return nil
}

func loadLegacyEnvs() {
	// For backward compatibility.
	instance.NavbarColor = loadEnv("DAGU__ADMIN_NAVBAR_COLOR", instance.NavbarColor)
	instance.NavbarTitle = loadEnv("DAGU__ADMIN_NAVBAR_TITLE", instance.NavbarTitle)
	instance.Port = loadEnv("DAGU__ADMIN_PORT", instance.Port)
	instance.Host = loadEnv("DAGU__ADMIN_HOST", instance.Host)
	instance.DataDir = loadEnv("DAGU__DATA", instance.DataDir)
	instance.LogDir = loadEnv("DAGU__DATA", instance.LogDir)
	instance.SuspendFlagsDir = loadEnv("DAGU__SUSPEND_FLAGS_DIR", instance.SuspendFlagsDir)
	instance.BaseConfig = loadEnv("DAGU__SUSPEND_FLAGS_DIR", instance.BaseConfig)
	instance.AdminLogsDir = loadEnv("DAGU__ADMIN_LOGS_DIR", instance.AdminLogsDir)
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
