package admin

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path"

	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagu/internal/utils"

	"gopkg.in/yaml.v2"
)

type Loader struct{}

func NewConfigLoader() *Loader {
	return &Loader{}
}

func DefaultConfig() (*Config, error) {
	c := &Config{}
	c.Init()
	err := c.setup()
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (cl *Loader) LoadAdminConfig(file string) (*Config, error) {

	if file == "" {
		homeDir := utils.MustGetUserHomeDir()
		file = path.Join(homeDir, ".dagu", "admin.yaml")
	}

	if !utils.FileExists(file) {
		return nil, ErrConfigNotFound
	}

	raw, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	def, err := cl.decode(raw)
	if err != nil {
		return nil, err
	}

	if def.Env == nil {
		def.Env = map[string]string{}
	}
	for k, v := range utils.DefaultEnv() {
		if _, ok := def.Env[v]; !ok {
			def.Env[k] = v
		}
	}

	c, err := buildFromDefinition(def)
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (cl *Loader) load(file string) (config map[string]interface{}, err error) {
	if !utils.FileExists(file) {
		return config, fmt.Errorf("file not found: %s", file)
	}
	return cl.readFile(file)
}

func (cl *Loader) readFile(file string) (config map[string]interface{}, err error) {
	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return cl.unmarshalData(data)
}

func (cl *Loader) unmarshalData(data []byte) (map[string]interface{}, error) {
	var cm map[string]interface{}

	err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&cm)
	if err != nil {
		return nil, err
	}
	return cm, nil
}

func (cl *Loader) decode(cm map[string]interface{}) (*configDefinition, error) {
	c := &configDefinition{}
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      c,
		TagName:     "",
	})
	err := md.Decode(cm)
	if err != nil {
		return nil, err
	}
	return c, nil
}

var ErrConfigNotFound = fmt.Errorf("admin.yaml file not found")
