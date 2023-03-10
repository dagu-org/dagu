package config

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/spf13/viper"
)

type Config struct {
	Host               string
	Port               string
	Env                []string
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
}

var C *Config = nil

func LoadConfig(homeDir string) error {
	appHome := path.Join(homeDir, appDir())

	log.Printf("Config file used: [%s]", viper.ConfigFileUsed())

	viper.AutomaticEnv()
	viper.SetEnvPrefix("dagu")

	viper.SetDefault("Host", "127.0.0.1")
	viper.SetDefault("Port", "8080")
	viper.SetDefault("Command", "dagu")
	viper.SetDefault("BaseConfig", path.Join(appHome, "config.yaml"))
	viper.SetDefault("LogDir", path.Join(appHome, "logs"))
	viper.SetDefault("DataDir", path.Join(appHome, "data"))
	viper.SetDefault("SuspendFlagsDir", path.Join(appHome, "suspend"))
	viper.SetDefault("AdminLogsDir", path.Join(appHome, "logs", "admin"))
	viper.SetDefault("DAGs", path.Join(appHome, "dags"))
	viper.SetDefault("NavbarColor", "")
	viper.SetDefault("NavbarTitle", "Dagu")

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

func appDir() string {
	appDir := os.ExpandEnv("${DAGU_HOME}")
	if appDir == "" {
		return ".dagu"
	}
	return appDir
}
