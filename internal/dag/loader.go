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
func Load(baseConfig, dag, params string) (*DAG, error) {
	return loadDAG(dag, BuildDAGOptions{
		baseConfig:   baseConfig,
		parameters:   params,
		metadataOnly: false,
		noEval:       false,
	})
}

// LoadWithoutEval loads config from file without evaluating env variables.
func LoadWithoutEval(dag string) (*DAG, error) {
	return loadDAG(dag, BuildDAGOptions{
		metadataOnly: false,
		noEval:       true,
	})
}

// LoadMetadata loads config from file and returns only the headline data.
func LoadMetadata(dag string) (*DAG, error) {
	return loadDAG(dag, BuildDAGOptions{
		metadataOnly: true,
		noEval:       true,
	})
}

// LoadData loads config from given data.
func LoadData(data []byte) (*DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		return nil, err
	}
	cdl := &configDefinitionLoader{}
	def, err := cdl.decode(raw)
	if err != nil {
		return nil, err
	}
	b := &Builder{
		options: BuildDAGOptions{metadataOnly: false, noEval: true},
	}
	return b.buildFromDefinition(def, nil)
}

func loadBaseConfig(file string, opts BuildDAGOptions) (*DAG, error) {
	if !util.FileExists(file) {
		return nil, nil
	}

	raw, err := load(file)
	if err != nil {
		return nil, err
	}

	cdl := &configDefinitionLoader{}
	def, err := cdl.decode(raw)
	if err != nil {
		return nil, err
	}

	buildOpts := opts
	buildOpts.metadataOnly = false
	b := &Builder{
		options: buildOpts,
	}
	return b.buildFromDefinition(def, nil)
}

func loadDAG(dag string, opts BuildDAGOptions) (*DAG, error) {
	file, err := prepareFilepath(dag)
	if err != nil {
		return nil, err
	}

	dst, err := loadBaseConfigIfRequired(opts.baseConfig, file, opts)
	if err != nil {
		return nil, err
	}

	raw, err := load(file)
	if err != nil {
		return nil, err
	}

	cdl := &configDefinitionLoader{}

	def, err := cdl.decode(raw)
	if err != nil {
		return nil, err
	}

	b := Builder{options: opts}
	c, err := b.buildFromDefinition(def, dst)

	if err != nil {
		return nil, err
	}

	err = cdl.merge(dst, c)
	if err != nil {
		return nil, err
	}

	dst.Location = file

	if !opts.noEval {
		dst.setup()
	}

	return dst, nil
}

// prepareFilepath prepares the filepath for the given file.
func prepareFilepath(f string) (string, error) {
	if f == "" {
		return "", errConfigFileRequired
	}
	if !strings.HasSuffix(f, ".yaml") && !strings.HasSuffix(f, ".yml") {
		f = fmt.Sprintf("%s.yaml", f)
	}
	return filepath.Abs(f)
}

// loadBaseConfigIfRequired loads the base config if needed, based on the given options.
func loadBaseConfigIfRequired(baseConfig, file string, opts BuildDAGOptions) (*DAG, error) {
	if !opts.metadataOnly && baseConfig != "" {
		dag, err := loadBaseConfig(baseConfig, opts)
		if err != nil {
			return nil, err
		}
		if dag != nil {
			return dag, nil
		}
	}
	return &DAG{Name: strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))}, nil
}

type mergeTransformer struct{}

var _ mergo.Transformers = (*mergeTransformer)(nil)

func (mt *mergeTransformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
	if typ == reflect.TypeOf(MailOn{}) {
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				dst.Set(src)
			}
			return nil
		}
	}
	return nil
}

func load(file string) (config map[string]interface{}, err error) {
	return readFile(file)
}

// readFile reads the contents of the file into a map.
func readFile(file string) (config map[string]interface{}, err error) {
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

// configDefinitionLoader is a helper struct to decode and merge configuration definitions.
type configDefinitionLoader struct{}

// decode decodes the configuration map into a configDefinition.
func (cdl *configDefinitionLoader) decode(cm map[string]interface{}) (*configDefinition, error) {
	c := &configDefinition{}
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		ErrorUnused: true,
		Result:      c,
		TagName:     "",
	})
	err := md.Decode(cm)
	return c, err
}

// merge merges the source DAG into the destination DAG.
func (cdl *configDefinitionLoader) merge(dst, src *DAG) error {
	err := mergo.Merge(dst, src, mergo.WithOverride,
		mergo.WithTransformers(&mergeTransformer{}))
	return err
}
