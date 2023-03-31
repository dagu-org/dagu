package dag

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/yohamta/dagu/internal/pipeline"
	"go.starlark.net/starlark"

	"github.com/imdario/mergo"
	"github.com/mitchellh/mapstructure"
	"github.com/yohamta/dagu/internal/utils"

	"gopkg.in/yaml.v2"
)

var ErrDAGNotFound = errors.New("DAG was not found")

// Loader is a config loader.
type Loader struct {
	BaseConfig string
}

// Load loads config from file.
func (cl *Loader) Load(f, params string) (*DAG, error) {
	return cl.loadDAGWithOptions(f, params, false, false, false)
}

// LoadwIithoutEval loads config from file without evaluating env variables.
func (cl *Loader) LoadWithoutEval(f string) (*DAG, error) {
	return cl.loadDAGWithOptions(f, "", false, true, true)
}

// LoadMetadataOnly loads config from file and returns only the headline data.
func (cl *Loader) LoadMetadataOnly(f string) (*DAG, error) {
	return cl.loadDAGWithOptions(f, "", true, true, true)
}

// loadDAGWithOptions loads the config file with the provided options.
func (cl *Loader) loadDAGWithOptions(f, params string, loadMetadataOnly, skipEnvEval, skipEnvSetup bool) (*DAG, error) {
	return cl.loadDAG(f,
		&BuildDAGOptions{
			parameters:       params,
			loadMetadataOnly: loadMetadataOnly,
			skipEnvEval:      skipEnvEval,
			skipEnvSetup:     skipEnvSetup,
		},
	)
}

// LoadData loads config from given data.
func (cl *Loader) LoadData(data []byte) (*DAG, error) {
	fl := &fileLoader{}
	raw, err := fl.unmarshalData(data)
	if err != nil {
		return nil, err
	}
	cdl := &configDefinitionLoader{}
	def, err := cdl.decode(raw)
	if err != nil {
		return nil, err
	}
	b := &DAGBuilder{
		options: BuildDAGOptions{loadMetadataOnly: false, skipEnvEval: true, skipEnvSetup: true},
	}
	return b.buildFromDefinition(def, nil)
}

func (cl *Loader) loadBaseConfig(file string, opts *BuildDAGOptions) (*DAG, error) {
	if !utils.FileExists(file) {
		return nil, nil
	}

	def, err := cl.loadFromYAML(file)
	if err != nil {
		return nil, err
	}

	buildOpts := *opts
	buildOpts.loadMetadataOnly = false
	buildOpts.defaultEnvs = utils.DefaultEnv()
	b := &DAGBuilder{
		options: buildOpts,
	}
	return b.buildFromDefinition(def, nil)
}

func (cl *Loader) loadDAG(f string, opts *BuildDAGOptions) (*DAG, error) {
	file, err := cl.prepareFilepath(f)
	if err != nil {
		return nil, err
	}

	dst, err := cl.loadBaseConfigIfRequired(file, opts)
	if err != nil {
		return nil, err
	}

	if dst == nil {
		dst = &DAG{}
	}

	ext := filepath.Ext(file)
	dst.Name = strings.TrimSuffix(filepath.Base(file), ext)

	var def *configDefinition
	if ext == ".star" {
		def, err = cl.loadFromStarLark(file)
	} else {
		def, err = cl.loadFromYAML(file)
	}

	if err != nil {
		return nil, err
	}

	b := DAGBuilder{options: *opts}
	c, err := b.buildFromDefinition(def, dst)

	if err != nil {
		return nil, err
	}

	err = merge(dst, c)
	if err != nil {
		return nil, err
	}

	dst.Location = file

	if !opts.skipEnvSetup {
		dst.setup()
	}

	return dst, nil
}

// prepareFilepath prepares the filepath for the given file.
func (cl *Loader) prepareFilepath(f string) (string, error) {
	if f == "" {
		return "", fmt.Errorf("config file was not specified")
	}
	if !strings.HasSuffix(f, ".yaml") && !strings.HasSuffix(f, ".yml") {
		f = fmt.Sprintf("%s.yaml", f)
	}
	return filepath.Abs(f)
}

func (cl *Loader) loadFromYAML(file string) (*configDefinition, error) {
	raw, err := cl.load(file)
	if err != nil {
		return nil, err
	}

	cdl := &configDefinitionLoader{}
	def, err := cdl.decode(raw)
	if err != nil {
		return nil, err
	}

	return def, nil
}

func (cl *Loader) loadFromStarLark(file string) (*configDefinition, error) {
	graph, err := pipeline.NewPipeline(file)
	if err != nil {
		return nil, err
	}
	c := &configDefinition{}
	c.Name = path.Base(file)
	nodes := graph["nodes"].(*starlark.Dict)
	edges := graph["edges"].(*starlark.List)
	dependencyMap := pipeline.EdgesToDependencyMap(edges)
	for _, name := range nodes.Keys() {
		value, found, err := nodes.Get(name)
		if err != nil {
			return nil, err
		}
		if !found {
			return nil, fmt.Errorf("node %v not found", name)
		}
		node := value.(*starlark.Dict)
		task, found, err := node.Get(starlark.String("task"))
		if task == nil || !found {
			return nil, fmt.Errorf("task of node %v missing", name)
		}
		if err != nil {
			return nil, err
		}
		taskDict := task.(*starlark.Dict)
		nameStr := name.(starlark.String).GoString()
		executor := lookupDict(taskDict, "executor")
		command := lookupDict(taskDict, "command")
		script := lookupDict(taskDict, "script")
		step := &stepDef{
			Name:     nameStr,
			Depends:  dependencyMap[nameStr],
			Executor: executor,
			Command:  command,
			Script:   script,
		}
		c.Steps = append(c.Steps, step)
	}
	return c, nil
}

// loadBaseConfigIfRequired loads the base config if needed, based on the given options.
func (cl *Loader) loadBaseConfigIfRequired(file string, opts *BuildDAGOptions) (*DAG, error) {
	if !opts.loadMetadataOnly && cl.BaseConfig != "" {
		dag, err := cl.loadBaseConfig(cl.BaseConfig, opts)
		if err != nil {
			return nil, err
		}
		if dag != nil {
			return dag, nil
		}
	}
	return &DAG{Name: strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))}, nil
}

func lookupDict(dict *starlark.Dict, name string) string {
	value, found, err := dict.Get(starlark.String(name))
	if !found {
		fmt.Printf("missing %v in dict", name)
	}
	if err != nil {
		fmt.Printf("lookup error: %v", err)
	}
	return value.(starlark.String).GoString()
}

type mergeTranformer struct{}

var _ mergo.Transformers = (*mergeTranformer)(nil)

func (mt *mergeTranformer) Transformer(typ reflect.Type) func(dst, src reflect.Value) error {
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

func (cl *Loader) load(file string) (config map[string]interface{}, err error) {
	return cl.readFile(file)
}

func (cl *Loader) readFile(file string) (config map[string]interface{}, err error) {
	fl := &fileLoader{}
	return fl.readFile(file)
}

// fileLoader is a helper struct to load and process configuration files.
type fileLoader struct{}

// readFile reads the contents of the file into a map.
func (fl *fileLoader) readFile(file string) (config map[string]interface{}, err error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %v", file, err)
	}
	return fl.unmarshalData(data)
}

// unmarshalData unmarshals the data from a byte slice into a map.
func (fl *fileLoader) unmarshalData(data []byte) (map[string]interface{}, error) {
	var cm map[string]interface{}
	err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&cm)
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
func merge(dst, src *DAG) error {
	err := mergo.Merge(dst, src, mergo.WithOverride,
		mergo.WithTransformers(&mergeTranformer{}))
	return err
}
