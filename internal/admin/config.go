package admin

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

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
}

func (c *Config) Init() {
	if c.Env == nil {
		c.Env = []string{}
	}
}

func (c *Config) setup() {
	if c.Command == "" {
		c.Command = "dagu"
	}
	if c.DAGs == "" {
		wd, err := os.Getwd()
		if err != nil {
			panic(err)
		}
		c.DAGs = wd
	}
	if c.Host == "" {
		c.Host = "127.0.0.1"
	}
	if c.Port == "" {
		c.Port = settings.MustGet(settings.CONFIG__ADMIN_PORT)
	}
	if len(c.Env) == 0 {
		env := utils.DefaultEnv()
		env, err := loadVariables(env)
		if err != nil {
			panic(err)
		}
		c.Env = buildConfigEnv(env)
	}
}

func buildFromDefinition(def *configDefinition) (c *Config, err error) {
	c = &Config{}
	c.Init()

	env, err := loadVariables(def.Env)
	if err != nil {
		return nil, err
	}
	c.Env = buildConfigEnv(env)

	c.Host, err = utils.ParseVariable(def.Host)
	if err != nil {
		return nil, err
	}
	c.Port = strconv.Itoa(def.Port)

	jd, err := utils.ParseVariable(def.Dags)
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(jd) {
		return nil, fmt.Errorf("DAGs directory should be absolute path. was %s", jd)
	}
	c.DAGs, err = filepath.Abs(jd)
	if err != nil {
		return nil, err
	}
	c.Command, err = utils.ParseVariable(def.Command)
	if err != nil {
		return nil, err
	}
	c.WorkDir, err = utils.ParseVariable(def.WorkDir)
	if err != nil {
		return nil, err
	}
	if c.WorkDir == "" {
		c.WorkDir, err = os.Getwd()
		if err != nil {
			return nil, err
		}
	}
	c.IsBasicAuth = def.IsBasicAuth
	c.BasicAuthUsername, err = utils.ParseVariable(def.BasicAuthUsername)
	if err != nil {
		return nil, err
	}
	c.BasicAuthPassword, err = utils.ParseVariable(def.BasicAuthPassword)
	if err != nil {
		return nil, err
	}
	c.LogEncodingCharset, err = utils.ParseVariable(def.LogEncodingCharset)
	if err != nil {
		return nil, err
	}
	return c, nil
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
