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

	"github.com/dagu-dev/dagu/internal/util"
	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"

	"gopkg.in/yaml.v2"
)

var (
	errConfigFileRequired = errors.New("config file was not specified")
	errReadFile           = errors.New("failed to read file")
)

// Load loads config from file.
func Load(base, dag, params string) (*DAG, error) {
	return loadDAG(dag, buildOpts{
		base:         base,
		parameters:   params,
		metadataOnly: false,
		noEval:       false,
	})
}

// LoadWithoutEval loads config from file without evaluating env variables.
func LoadWithoutEval(dag string) (*DAG, error) {
	return loadDAG(dag, buildOpts{
		metadataOnly: false,
		noEval:       true,
	})
}

// LoadMetadata loads config from file and returns only the headline data.
func LoadMetadata(dag string) (*DAG, error) {
	return loadDAG(dag, buildOpts{
		metadataOnly: true,
		noEval:       true,
	})
}

// LoadYAML loads config from YAML data.
func LoadYAML(data []byte) (*DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		return nil, err
	}

	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	b := &builder{opts: buildOpts{metadataOnly: false, noEval: true}}
	return b.build(def, nil)
}

// loadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the DAG configuration.
func loadBaseConfig(file string, opts buildOpts) (*DAG, error) {
	// The base config is optional.
	if !util.FileExists(file) {
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

	// Build the DAG from the config definition.
	// Base configuration must load all the data.
	buildOpts := opts
	buildOpts.metadataOnly = false

	b := &builder{opts: buildOpts}
	return b.build(def, nil)
}

// loadDAG loads the DAG from the given file.
func loadDAG(dag string, opts buildOpts) (*DAG, error) {
	// Find the absolute path to the file.
	// The file must be a YAML file.
	file, err := prepareFilepath(dag)
	if err != nil {
		return nil, err
	}

	// Load the base configuration unless only the metadata is required.
	// If only the metadata is required, the base configuration is not loaded
	// and the DAG is created with the default values.
	dst, err := loadBaseConfigIfRequired(opts.base, file, opts)
	if err != nil {
		return nil, err
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

	// Set the absolute path to the file.
	dst.Location = file

	// Set the default values for the DAG.
	if !opts.metadataOnly {
		dst.setup()
	}

	return dst, nil
}

// prepareFilepath prepares the filepath for the given file.
// The file must be a YAML file.
func prepareFilepath(file string) (string, error) {
	if file == "" {
		return "", errConfigFileRequired
	}

	// The file name can be specified without the extension.
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		file = fmt.Sprintf("%s.yaml", file)
	}

	return filepath.Abs(file)
}

// loadBaseConfigIfRequired loads the base config if needed, based on the given options.
func loadBaseConfigIfRequired(baseConfig, file string, opts buildOpts) (*DAG, error) {
	if !opts.metadataOnly && baseConfig != "" {
		dag, err := loadBaseConfig(baseConfig, opts)
		if err != nil {
			return nil, err
		}
		// Base config is optional.
		if dag != nil {
			return dag, nil
		}
	}

	// return a DAG with the default values.
	dagName := strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	return &DAG{Name: dagName}, nil
}

type mergeTransformer struct{}

var _ mergo.Transformers = (*mergeTransformer)(nil)

func (mt *mergeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	// mergo does not overwrite a value with zero value for a pointer.
	if typ == reflect.TypeOf(MailOn{}) {
		// We need to explicitly overwrite the value for a pointer with a zero value.
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
func readFile(file string) (config map[string]any, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%w %s: %v", errReadFile, file, err)
	}

	return unmarshalData(data)
}

// unmarshalData unmarshals the data into a map.
func unmarshalData(data []byte) (map[string]interface{}, error) {
	var cm map[string]interface{}
	err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&cm)
	if errors.Is(err, io.EOF) {
		err = nil
	}

	return cm, err
}

// decode decodes the configuration map into a configDefinition.
func decode(cm map[string]interface{}) (*definition, error) {
	c := &definition{}
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
