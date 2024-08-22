// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package dag

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/daguflow/dagu/internal/util"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"

	"gopkg.in/yaml.v2"
)

var (
	errConfigFileRequired = errors.New("config file was not specified")
	errReadFile           = errors.New("failed to read file")
)

// Load loads config from file.
func Load(baseDAGFile, dAGFile, params string) (*DAG, error) {
	return loadFile(dAGFile, buildOpts{
		base:         baseDAGFile,
		parameters:   params,
		metadataOnly: false,
		noEval:       false,
		file:         dAGFile,
	})
}

// LoadWithoutEval loads config from file without evaluating env variables.
func LoadWithoutEval(dAGFile string) (*DAG, error) {
	return loadFile(dAGFile, buildOpts{
		metadataOnly: false,
		noEval:       true,
		file:         dAGFile,
	})
}

// LoadMetadata loads config from file and returns only the headline data.
func LoadMetadata(dAGFile string) (*DAG, error) {
	return loadFile(dAGFile, buildOpts{
		metadataOnly: true,
		noEval:       true,
		file:         dAGFile,
	})
}

// LoadYAML loads config from YAML data.
// It does not evaluate the environment variables.
// This is used to validate the YAML data.
func LoadYAML(name string, base []byte, source []byte) (*DAG, error) {
	return loadDAG(base, source, buildOpts{
		metadataOnly: false,
		noEval:       true,
		name:         name,
	})
}

func loadFile(file string, opts buildOpts) (*DAG, error) {
	var baseData []byte

	if opts.base != "" {
		// Find the absolute path to the file.
		// The file must be a YAML file.
		base, err := normalizeFilePath(opts.base)
		if err != nil {
			return nil, err
		}
		if util.FileExists(base) {
			// Load the base configuration if it exists.
			baseData, err = os.ReadFile(base)
			if err != nil {
				return nil, fmt.Errorf("%w %s: %v", errReadFile, base, err)
			}
		}
	}

	// Load the DAG from the file.
	file, err := normalizeFilePath(file)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %v", errReadFile, file, err)
	}

	return loadDAG(baseData, data, opts)
}

// loadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the DAG configuration.
func loadBaseConfig(base []byte, opts buildOpts) (*DAG, error) {
	raw, err := unmarshalData(base)
	if err != nil {
		return nil, err
	}

	// Decode the raw data into a config definition.
	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	// Build the DAG from the config definition.
	// Base configuration must load all the data.
	buildOpts := opts
	buildOpts.metadataOnly = false

	b := &builder{opts: buildOpts}
	return b.build(def, nil)
}

// loadDAG loads the DAG from the given file.
func loadDAG(base, data []byte, opts buildOpts) (*DAG, error) {
	// Load the base configuration unless only the metadata is required.
	// If only the metadata is required, the base configuration is not loaded
	// and the DAG is created with the default values.
	var dst *DAG
	if base != nil {
		baseDAG, err := loadBaseConfig(base, opts)
		if err != nil {
			return nil, err
		}
		// Base config is optional.
		if baseDAG != nil {
			dst = baseDAG
		}
	} else {
		dst = new(DAG)
	}

	raw, err := unmarshalData(data)
	if err != nil {
		return nil, err
	}

	// Decode the raw data into a config definition.
	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	// Build the DAG from the config definition.
	b := builder{opts: opts}
	c, err := b.build(def, dst.Env)
	if err != nil {
		return nil, err
	}

	// Merge the DAG with the base configuration.
	// The DAG configuration overrides the base configuration.
	err = merge(dst, c)
	if err != nil {
		return nil, err
	}

	// Set the default values for the DAG.
	if !opts.metadataOnly {
		dst.setup()
	}

	dst.Source.Base = string(base)
	dst.Source.Source = string(data)

	// Check if the DAG has the required fields.
	if err := dst.validate(); err != nil {
		return nil, err
	}

	return dst, nil
}

// getDefaultName returns the default name for the given file.
// The default name is the filename without the extension.
func getDefaultName(file string) string {
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

// normalizeFilePath prepares the filepath for the given file.
// The file must be a YAML file.
func normalizeFilePath(file string) (string, error) {
	if file == "" {
		return "", errConfigFileRequired
	}

	// The file name can be specified without the extension.
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		file = fmt.Sprintf("%s.yaml", file)
	}

	return filepath.Abs(file)
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
