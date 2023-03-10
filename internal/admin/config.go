package admin

import (
	"fmt"
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
	appHome := path.Join(homeDir, ".dagu")

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
	viper.SetDefault("NavbarTitle", "Dagu")

	cfg := &Config{}
	err := viper.Unmarshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to unmarshal Config file: %w", err)
	}
	C = cfg
	return nil
}
