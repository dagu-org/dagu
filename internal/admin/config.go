package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
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
	BaseConfig         string
	NavbarColor        string
	NavbarTitle        string
}

func newConfig() *Config {
	return &Config{
		Env: []string{},
	}
}

func DefaultConfig() (*Config, error) {
	cfg := newConfig()
	err := cfg.setup()
	return cfg, err
}

func defaultConfig() *Config {
	return &Config{
		Command:     "dagu",
		DAGs:        settings.MustGet(settings.SETTING__ADMIN_DAGS_DIR),
		LogDir:      settings.MustGet(settings.SETTING__ADMIN_LOGS_DIR),
		Host:        "127.0.0.1",
		Port:        settings.MustGet(settings.SETTING__ADMIN_PORT),
		NavbarColor: settings.MustGet(settings.SETTING__ADMIN_NAVBAR_COLOR),
		NavbarTitle: settings.MustGet(settings.SETTING__ADMIN_NAVBAR_TITLE),
	}
}

func (cfg *Config) setup() error {
	// TODO: refactor to avoid loading unnecessary configs
	def := defaultConfig()
	setDef := func(val *string, def string) {
		*val = utils.StringWithFallback(*val, def)
	}
	setDef(&cfg.Command, def.Command)
	setDef(&cfg.DAGs, def.DAGs)
	setDef(&cfg.LogDir, def.LogDir)
	setDef(&cfg.Host, def.Host)
	setDef(&cfg.Port, def.Port)
	setDef(&cfg.NavbarColor, def.NavbarColor)
	setDef(&cfg.NavbarTitle, def.NavbarTitle)

	if len(cfg.Env) == 0 {
		env := utils.DefaultEnv()
		env, err := loadVariables(env)
		if err != nil {
			return err
		}
		cfg.Env = buildConfigEnv(env)
	}
	return nil
}

func buildFromDefinition(def *configDefinition) (cfg *Config, err error) {
	cfg = newConfig()

	for _, fn := range []func(cfg *Config, def *configDefinition) error{
		func(cfg *Config, def *configDefinition) error {
			env, err := loadVariables(def.Env)
			if err != nil {
				return err
			}
			cfg.Env = buildConfigEnv(env)
			return nil
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.Host, err = utils.ParseVariable(def.Host)
			if def.Port != 0 {
				cfg.Port = strconv.Itoa(def.Port)
			}
			return err
		},
		func(cfg *Config, def *configDefinition) (err error) {
			val, err := utils.ParseVariable(def.Dags)
			if err == nil && len(val) > 0 {
				if !filepath.IsAbs(val) {
					return fmt.Errorf("DAGs directory should be absolute path. was %s", val)
				}
				cfg.DAGs, err = filepath.Abs(val)
				if err != nil {
					return fmt.Errorf("failed to resolve DAGs directory: %w", err)
				}
			}
			return err
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.Command, err = utils.ParseVariable(def.Command)
			return err
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.WorkDir, err = utils.ParseVariable(def.WorkDir)
			if err == nil && strings.TrimSpace(cfg.WorkDir) == "" {
				cfg.WorkDir, err = os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to resolve working directory: %w", err)
				}
			}
			return err
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.BasicAuthUsername, err = utils.ParseVariable(def.BasicAuthUsername)
			if err != nil {
				return err
			}
			cfg.BasicAuthPassword, err = utils.ParseVariable(def.BasicAuthPassword)
			if err != nil {
				return err
			}
			return nil
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.LogEncodingCharset, err = utils.ParseVariable(def.LogEncodingCharset)
			return err
		},
		func(cfg *Config, def *configDefinition) (err error) {
			cfg.BaseConfig, err = utils.ParseVariable(strings.TrimSpace(def.BaseConfig))
			if err != nil {
				return err
			}
			if cfg.BaseConfig == "" {
				cfg.BaseConfig = settings.MustGet(settings.SETTING__BASE_CONFIG)
			}
			return nil
		},
		func(cfg *Config, def *configDefinition) error {
			cfg.NavbarColor = def.NavbarColor
			cfg.NavbarTitle = def.NavbarTitle
			return nil
		},
	} {
		if err := fn(cfg, def); err != nil {
			return nil, err
		}
	}

	cfg.LogDir = def.LogDir
	cfg.IsBasicAuth = def.IsBasicAuth

	return cfg, nil
}

func buildConfigEnv(vars map[string]string) []string {
	ret := []string{}
	for k, v := range vars {
		ret = append(ret, fmt.Sprintf("%s=%s", k, v))
	}
	return ret
}

func loadVariables(strVariables map[string]string) (map[string]string, error) {
	vars := map[string]string{}
	for k, v := range strVariables {
		parsed, err := utils.ParseVariable(v)
		if err != nil {
			return nil, err
		}
		vars[k] = parsed
		err = os.Setenv(k, parsed)
		if err != nil {
			return nil, err
		}
	}
	return vars, nil
}
