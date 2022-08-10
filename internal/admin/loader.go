package admin

import (
	"bytes"
	"fmt"
	"os"

	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagu/internal/utils"

	"gopkg.in/yaml.v2"
)

type Loader struct{}

func DefaultConfig() *Config {
	c := &Config{}
	c.Init()
	c.setup()
	return c
}

func (cl *Loader) LoadAdminConfig(file string) (*Config, error) {
	if !utils.FileExists(file) {
		return nil, ErrConfigNotFound
	}

	var (
		raw map[string]interface{} = nil
		def *configDefinition      = nil
		cfg *Config                = nil
	)

	for _, fn := range []func() error{
		func() (err error) {
			raw, err = cl.load(file)
			return err
		},
		func() (err error) {
			def, err = cl.decode(raw)
			return err
		},
		func() (err error) {
			if def.Env == nil {
				def.Env = map[string]string{}
			}

			for k, v := range utils.DefaultEnv() {
				if _, ok := def.Env[v]; !ok {
					def.Env[k] = v
				}
			}
			return nil
		},
		func() (err error) {
			cfg, err = buildFromDefinition(def)
			return err
		},
	} {
		if err := fn(); err != nil {
			return nil, err
		}
	}

	cfg.setup()
	return cfg, nil
}

func (cl *Loader) load(file string) (config map[string]interface{}, err error) {
	return cl.readFile(file)
}

func (cl *Loader) readFile(file string) (config map[string]interface{}, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return cl.unmarshalData(data)
}

func (cl *Loader) unmarshalData(data []byte) (map[string]interface{}, error) {
	var cm map[string]interface{}
	var err error
	if len(data) > 0 {
		err = yaml.NewDecoder(bytes.NewReader(data)).Decode(&cm)
	}
	return cm, err
}

func (cl *Loader) decode(cm map[string]interface{}) (*configDefinition, error) {
	c := &configDefinition{}
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      c,
		TagName:     "",
	})
	err := md.Decode(cm)
	return c, err
}

var ErrConfigNotFound = fmt.Errorf("admin.yaml file not found")
