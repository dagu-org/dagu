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
}

func (cfg *Config) Init() {
	if cfg.Env == nil {
		cfg.Env = []string{}
	}
}

func (cfg *Config) setup() {
	cfg.Command = utils.StringWithFallback(cfg.Command, "dagu")
	cfg.DAGs = utils.StringWithFallback(cfg.DAGs,
		settings.MustGet(settings.SETTING__ADMIN_DAGS_DIR))
	cfg.LogDir = utils.StringWithFallback(cfg.LogDir,
		settings.MustGet(settings.SETTING__ADMIN_LOGS_DIR))
	cfg.Host = utils.StringWithFallback(cfg.Host, "127.0.0.1")
	cfg.Port = utils.StringWithFallback(cfg.Port,
		settings.MustGet(settings.SETTING__ADMIN_PORT))
	if len(cfg.Env) == 0 {
		env := utils.DefaultEnv()
		env, err := loadVariables(env)
		if err != nil {
			panic(err)
		}
		cfg.Env = buildConfigEnv(env)
	}
}

func buildFromDefinition(def *configDefinition) (cfg *Config, err error) {
	cfg = &Config{}
	cfg.Init()

	for _, fn := range []func(cfg *Config, def *configDefinition) error{
		buildEnvs,
		buildHostPort,
		buildDAGsDir,
		buildCommand,
		buildWorkDir,
		buildBasicAuthOpts,
		buidEncodingOpts,
	} {
		if err := fn(cfg, def); err != nil {
			return nil, err
		}
	}

	cfg.LogDir = def.LogDir
	cfg.IsBasicAuth = def.IsBasicAuth

	return cfg, nil
}

func buildEnvs(cfg *Config, def *configDefinition) error {
	env, err := loadVariables(def.Env)
	if err != nil {
		return err
	}
	cfg.Env = buildConfigEnv(env)
	return nil
}

func buildHostPort(cfg *Config, def *configDefinition) (err error) {
	cfg.Host, err = utils.ParseVariable(def.Host)
	if def.Port == 0 {
		cfg.Port = settings.MustGet(settings.SETTING__ADMIN_PORT)
	} else {
		cfg.Port = strconv.Itoa(def.Port)
	}
	return err
}

func buildDAGsDir(cfg *Config, def *configDefinition) (err error) {
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
}

func buildCommand(cfg *Config, def *configDefinition) (err error) {
	cfg.Command, err = utils.ParseVariable(def.Command)
	return err
}

func buildWorkDir(cfg *Config, def *configDefinition) (err error) {
	cfg.WorkDir, err = utils.ParseVariable(def.WorkDir)
	if err == nil && strings.TrimSpace(cfg.WorkDir) == "" {
		cfg.WorkDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to resolve working directory: %w", err)
		}
	}
	return err
}

func buildBasicAuthOpts(cfg *Config, def *configDefinition) (err error) {
	cfg.BasicAuthUsername, err = utils.ParseVariable(def.BasicAuthUsername)
	if err != nil {
		return err
	}
	cfg.BasicAuthPassword, err = utils.ParseVariable(def.BasicAuthPassword)
	if err != nil {
		return err
	}
	return nil
}

func buidEncodingOpts(cfg *Config, def *configDefinition) (err error) {
	cfg.LogEncodingCharset, err = utils.ParseVariable(def.LogEncodingCharset)
	return err
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
