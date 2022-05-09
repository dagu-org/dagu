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
	"github.com/yohamta/dagu/internal/utils"

	"gopkg.in/yaml.v2"
)

var ErrConfigNotFound = errors.New("config file was not found")

type Loader struct {
	HomeDir string
}

func (cl *Loader) Load(f, params string) (*Config, error) {
	return cl.loadConfig(f, params, false, false)
}

func (cl *Loader) LoadWithoutEval(f string) (*Config, error) {
	return cl.loadConfig(f, "", false, true)
}

func (cl *Loader) LoadHeadOnly(f string) (*Config, error) {
	return cl.loadConfig(f, "", true, true)
}

func (cl *Loader) LoadData(data []byte) (*Config, error) {
	raw, err := cl.unmarshalData(data)
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
	return buildFromDefinition(
		def, nil, &BuildConfigOptions{headOnly: false, noEval: true},
	)
}

func (cl *Loader) loadGlobalConfig(file string) (*Config, error) {
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

	for k, v := range utils.DefaultEnv() {
		if _, ok := def.Env[v]; !ok {
			def.Env[k] = v
		}
	}

	return buildFromDefinition(
		def, nil, &BuildConfigOptions{headOnly: false},
	)
}

func (cl *Loader) loadConfig(f, params string, headOnly bool, noEval bool) (*Config, error) {
	if f == "" {
		return nil, fmt.Errorf("config file was not specified")
	}
	file, err := filepath.Abs(f)
	if err != nil {
		return nil, err
	}

	var dst *Config = nil

	if !headOnly {
		file := path.Join(cl.HomeDir, ".dagu/config.yaml")
		dst, err = cl.loadGlobalConfig(file)
		if err != nil {
			return nil, err
		}
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

	c, err := buildFromDefinition(def, dst,
		&BuildConfigOptions{
			headOnly:   headOnly,
			parameters: params,
			noEval:     noEval,
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

func (cl *Loader) merge(dst, src *Config) error {
	return mergo.MergeWithOverwrite(dst, src)
}

func (cl *Loader) load(file string) (config map[string]interface{}, err error) {
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
