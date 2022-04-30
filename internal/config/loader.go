package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"path/filepath"

	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagman/internal/utils"

	"gopkg.in/yaml.v2"
)

var ErrConfigNotFound = errors.New("config file was not found")

type Loader struct {
	dir     string
	homeDir string
}

func NewConfigLoader() *Loader {
	return &Loader{
		homeDir: utils.MustGetUserHomeDir(),
		dir:     utils.MustGetwd(),
	}
}

func (cl *Loader) Load(f, params string) (*Config, error) {
	file, err := filepath.Abs(f)
	if err != nil {
		return nil, err
	}

	dst, err := cl.LoadGlobalConfig()
	if err != nil {
		return nil, err
	}
	if dst == nil {
		dst = &Config{}
		dst.Init()
	}

	raw, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	def, err := cl.decode(raw)
	if err != nil {
		return nil, err
	}

	if err := assertDef(def); err != nil {
		return nil, err
	}

	c, err := buildFromDefinition(def, file,
		dst,
		&BuildConfigOptions{
			headOnly:   false,
			parameters: params,
		})
	if err != nil {
		return nil, err
	}

	err = cl.merge(dst, c)
	if err != nil {
		return nil, err
	}

	dst.setup(file)

	return dst, nil
}

func (cl *Loader) LoadHeadOnly(f string) (*Config, error) {
	file, err := filepath.Abs(f)
	if err != nil {
		return nil, err
	}

	raw, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	def, err := cl.decode(raw)
	if err != nil {
		return nil, err
	}

	if err := assertDef(def); err != nil {
		return nil, err
	}

	c, err := buildFromDefinition(def, file, nil,
		&BuildConfigOptions{
			headOnly: true,
		})
	if err != nil {
		return nil, err
	}

	c.setup(file)

	return c, nil
}

func (cl *Loader) LoadGlobalConfig() (*Config, error) {
	if cl.homeDir == "" {
		return nil, fmt.Errorf("home directory was not found.")
	}

	file := path.Join(cl.homeDir, ".dagman", "config.yaml")
	if !utils.FileExists(file) {
		return nil, nil
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

	c, err := buildFromDefinition(
		def, file, nil,
		&BuildConfigOptions{headOnly: false},
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}

func (cl *Loader) merge(dst, src *Config) error {
	if err := mergo.MergeWithOverwrite(dst, src); err != nil {
		return err
	}
	return nil
}

func (cl *Loader) load(file string) (config map[string]interface{}, err error) {
	if !utils.FileExists(file) {
		return config, ErrConfigNotFound
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
