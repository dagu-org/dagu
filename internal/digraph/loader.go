// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package digraph

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"

	"gopkg.in/yaml.v2"
)

type LoadOptions struct {
	baseDAG      string
	params       string
	paramsList   []string
	noEval       bool
	onlyMetadata bool
}

type LoadOption func(*LoadOptions)

func WithBaseConfig(baseDAG string) LoadOption {
	return func(o *LoadOptions) {
		o.baseDAG = baseDAG
	}
}

func WithParams(params any) LoadOption {
	return func(o *LoadOptions) {
		switch params := params.(type) {
		case string:
			o.params = params
		case []string:
			o.paramsList = params
		default:
			panic(fmt.Sprintf("invalid type %T for params", params))
		}
	}
}

func WithoutEval() LoadOption {
	return func(o *LoadOptions) {
		o.noEval = true
	}
}

func OnlyMetadata() LoadOption {
	return func(o *LoadOptions) {
		o.onlyMetadata = true
	}
}

// Load load the DAG from the given file.
func Load(ctx context.Context, dag string, opts ...LoadOption) (*DAG, error) {
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	return loadDAG(ctx, dag, buildOpts{
		base:           options.baseDAG,
		parameters:     options.params,
		parametersList: options.paramsList,
		onlyMetadata:   options.onlyMetadata,
		noEval:         options.noEval,
	})
}

// LoadWithoutEval loads config without evaluating dynamic fields.
func LoadWithoutEval(ctx context.Context, dag string) (*DAG, error) {
	return loadDAG(ctx, dag, buildOpts{
		onlyMetadata: false,
		noEval:       true,
	})
}

// LoadMetadata loads only basic information from the DAG.
// E.g. name, description, schedule, etc.
func LoadMetadata(ctx context.Context, dag string) (*DAG, error) {
	return loadDAG(ctx, dag, buildOpts{
		onlyMetadata: true,
		noEval:       true,
	})
}

// LoadYAML loads config from YAML data.
// It does not evaluate the environment variables.
// This is used to validate the YAML data.
func LoadYAML(ctx context.Context, data []byte) (*DAG, error) {
	return loadYAML(ctx, data, buildOpts{
		onlyMetadata: false,
		noEval:       true,
	})
}

// LoadYAML loads config from YAML data.
func loadYAML(ctx context.Context, data []byte, opts buildOpts) (*DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		return nil, err
	}

	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	return build(ctx, def, opts, nil)
}

// loadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the DAG configuration.
func loadBaseConfig(ctx context.Context, file string, opts buildOpts) (*DAG, error) {
	// The base config is optional.
	if !fileutil.FileExists(file) {
		return nil, nil
	}

	// Load the raw data from the file.
	raw, err := readFile(file)
	if err != nil {
		return nil, err
	}

	// Decode the raw data into a config definition.
	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	return build(ctx, def, buildOpts{noEval: opts.noEval}, nil)
}

// loadDAG loads the DAG from the given file.
func loadDAG(ctx context.Context, dag string, opts buildOpts) (*DAG, error) {
	filePath, err := resolveYamlFilePath(dag)
	if err != nil {
		return nil, err
	}

	dest, err := loadBaseConfigIfRequired(ctx, opts.base, opts)
	if err != nil {
		return nil, err
	}

	raw, err := readFile(filePath)
	if err != nil {
		return nil, err
	}

	spec, err := decode(raw)
	if err != nil {
		return nil, err
	}

	target, err := build(ctx, spec, opts, dest.Env)
	if err != nil {
		return nil, err
	}

	// Merge the target DAG into the dest DAG.
	err = merge(dest, target)
	if err != nil {
		return nil, err
	}

	// Set the absolute path to the file.
	dest.Location = filePath

	// Set the name if not set.
	if dest.Name == "" {
		dest.Name = defaultName(filePath)
	}

	// Set defaults
	dest.setup()

	return dest, nil
}

// defaultName returns the default name for the given file.
// The default name is the filename without the extension.
func defaultName(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

var errConfigFileRequired = errors.New("config file was not specified")

func resolveYamlFilePath(file string) (string, error) {
	if file == "" {
		return "", errConfigFileRequired
	}

	// The file name can be specified without the extension.
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		file = fmt.Sprintf("%s.yaml", file)
	}

	return filepath.Abs(file)
}

// loadBaseConfigIfRequired loads the base config if needed, based on the
// given options.
func loadBaseConfigIfRequired(ctx context.Context, baseConfig string, opts buildOpts) (*DAG, error) {
	if !opts.onlyMetadata && baseConfig != "" {
		dag, err := loadBaseConfig(ctx, baseConfig, opts)
		if err != nil {
			// Failed to load the base config.
			return nil, err
		}
		if dag != nil {
			// Found the base config.
			return dag, nil
		}
	}

	// No base config.
	return new(DAG), nil
}

type mergeTransformer struct{}

var _ mergo.Transformers = (*mergeTransformer)(nil)

func (*mergeTransformer) Transformer(
	typ reflect.Type,
) func(dst, src reflect.Value) error {
	// mergo does not overwrite a value with zero value for a pointer.
	if typ == reflect.TypeOf(MailOn{}) {
		// We need to explicitly overwrite the value for a pointer with a zero
		// value.
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				dst.Set(src)
			}

			return nil
		}
	}

	return nil
}

// readFile reads the contents of the file into a map.
func readFile(file string) (cfg map[string]any, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", file, err)
	}

	return unmarshalData(data)
}

// unmarshalData unmarshals the data into a map.
func unmarshalData(data []byte) (map[string]any, error) {
	var cm map[string]any
	err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&cm)
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return cm, err
}

// decode decodes the configuration map into a configDefinition.
func decode(cm map[string]any) (*definition, error) {
	c := new(definition)
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      c,
		TagName:     "",
	})
	err := md.Decode(cm)

	return c, err
}

// merge merges the source DAG into the destination DAG.
func merge(dst, src *DAG) error {
	return mergo.Merge(dst, src, mergo.WithOverride,
		mergo.WithTransformers(&mergeTransformer{}))
}
