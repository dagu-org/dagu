package digraph

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"dario.cat/mergo"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/go-viper/mapstructure/v2"

	"github.com/goccy/go-yaml"
)

// Errors for loading DAGs
var (
	ErrNameOrPathRequired = errors.New("name or path is required")
	ErrInvalidJSONFile    = errors.New("invalid JSON file")
)

// LoadOptions contains options for loading a DAG.
type LoadOptions struct {
	name         string   // Name of the DAG.
	baseConfig   string   // Path to the base DAG configuration file.
	params       string   // Parameters to override default parameters in the DAG.
	paramsList   []string // List of parameters to override default parameters in the DAG.
	noEval       bool     // Flag to disable evaluation of dynamic fields.
	onlyMetadata bool     // Flag to load only metadata without full DAG details.
	dagsDir      string   // Directory containing the DAG files.
}

// LoadOption is a function type for setting LoadOptions.
type LoadOption func(*LoadOptions)

// WithBaseConfig sets the base DAG configuration file.
func WithBaseConfig(baseDAG string) LoadOption {
	return func(o *LoadOptions) {
		o.baseConfig = baseDAG
	}
}

// WithParams sets the parameters for the DAG.
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

// WithoutEval disables the evaluation of dynamic fields.
func WithoutEval() LoadOption {
	return func(o *LoadOptions) {
		o.noEval = true
	}
}

// OnlyMetadata sets the flag to load only metadata.
func OnlyMetadata() LoadOption {
	return func(o *LoadOptions) {
		o.onlyMetadata = true
	}
}

// WithName sets the name of the DAG.
func WithName(name string) LoadOption {
	return func(o *LoadOptions) {
		o.name = name
	}
}

// WithDAGsDir sets the directory containing the DAG files.
// This directory is used as the base path for resolving relative DAG file paths.
// When a DAG is loaded by name rather than absolute path, the system will look
// for the DAG file in this directory. If not specified, the current working
// directory is used as the default.
func WithDAGsDir(dagsDir string) LoadOption {
	return func(o *LoadOptions) {
		o.dagsDir = dagsDir
	}
}

// Load loads a Directed Acyclic Graph (DAG) from a file path or name with the given options.
//
// The function handles different input formats:
//
// 1. Absolute paths:
//   - JSON files (.json): Loaded directly without dynamic evaluation
//   - YAML files (.yaml/.yml): Processed with dynamic evaluation, including base configs,
//     parameters, and environment variables
//
// 2. Relative paths or filenames:
//   - Resolved against the DAGsDir specified in options
//   - If DAGsDir is not provided, the current working directory is used
//   - For YAML files, the extension is optional
//
// This approach provides a flexible way to load DAG definitions from multiple sources
// while supporting customization through the LoadOptions.
func Load(ctx context.Context, nameOrPath string, opts ...LoadOption) (*DAG, error) {
	if nameOrPath == "" {
		return nil, ErrNameOrPathRequired
	}

	// If the nameOrPath is an absolute path to JSON file, load it directly.
	if filepath.IsAbs(nameOrPath) && strings.HasSuffix(nameOrPath, ".json") {
		raw, err := readRawFile(nameOrPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read JSON file %q: %w", nameOrPath, err)
		}
		var dag *DAG
		if err := json.Unmarshal(raw, &dag); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON file %q: %w", nameOrPath, err)
		}
		// We keep the 'location' field as is in the JSON file.
		// The 'location' field is used to create proper unix socket path.
		return dag, nil
	}

	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	buildContext := BuildContext{
		ctx: ctx,
		opts: BuildOpts{
			Base:           options.baseConfig,
			Parameters:     options.params,
			ParametersList: options.paramsList,
			OnlyMetadata:   options.onlyMetadata,
			NoEval:         options.noEval,
			Name:           options.name,
			DAGsDir:        options.dagsDir,
		},
	}
	return loadDAG(buildContext, nameOrPath)
}

// LoadYAML loads the DAG from the given YAML data with the specified options.
func LoadYAML(ctx context.Context, data []byte, opts ...LoadOption) (*DAG, error) {
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	return LoadYAMLWithOpts(ctx, data, BuildOpts{
		Base:           options.baseConfig,
		Parameters:     options.params,
		ParametersList: options.paramsList,
		OnlyMetadata:   options.onlyMetadata,
		NoEval:         options.noEval,
		Name:           options.name,
		DAGsDir:        options.dagsDir,
	})
}

// LoadYAMLWithOpts loads the DAG configuration from YAML data.
func LoadYAMLWithOpts(ctx context.Context, data []byte, opts BuildOpts) (*DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		return nil, ErrorList{err}
	}

	def, err := decode(raw)
	if err != nil {
		return nil, ErrorList{err}
	}

	return build(BuildContext{ctx: ctx, opts: opts}, def)
}

// LoadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the DAG configuration.
func LoadBaseConfig(ctx BuildContext, file string) (*DAG, error) {
	// The base config is optional.
	if !fileutil.FileExists(file) {
		return nil, nil
	}

	// Load the raw data from the file.
	raw, err := readYAMLFile(file)
	if err != nil {
		return nil, err
	}

	// Decode the raw data into a config definition.
	def, err := decode(raw)
	if err != nil {
		return nil, ErrorList{err}
	}

	ctx = ctx.WithOpts(BuildOpts{NoEval: ctx.opts.NoEval}).WithFile(file)
	dag, err := build(ctx, def)

	if err != nil {
		return nil, ErrorList{err}
	}
	return dag, nil
}

// loadDAG loads the DAG from the given file.
func loadDAG(ctx BuildContext, nameOrPath string) (*DAG, error) {
	filePath, err := resolveYamlFilePath(ctx, nameOrPath)
	if err != nil {
		return nil, err
	}

	ctx = ctx.WithFile(filePath)

	dest, err := loadBaseConfigIfRequired(ctx, ctx.opts.Base)
	if err != nil {
		return nil, err
	}

	raw, err := readYAMLFile(filePath)
	if err != nil {
		return nil, err
	}

	spec, err := decode(raw)
	if err != nil {
		return nil, err
	}

	target, err := build(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Merge the target DAG into the dest DAG.
	dest.Location = "" // No need to set the location for the base config.
	err = merge(dest, target)
	if err != nil {
		return nil, err
	}

	dest.initializeDefaults()

	return dest, nil
}

// defaultName returns the default name for the given file.
// The default name is the filename without the extension.
func defaultName(file string) string {
	if file == "" {
		return ""
	}
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}

// resolveYamlFilePath resolves the YAML file path.
// If the file name does not have an extension, it appends ".yaml".
func resolveYamlFilePath(ctx BuildContext, file string) (string, error) {
	if file == "" {
		return "", errors.New("file path is required")
	}

	if filepath.IsAbs(file) {
		// If the file is an absolute path, return it as is.
		return file, nil
	}

	// Check if the file exists in the current Directory.
	absFile, err := filepath.Abs(file)
	if err == nil && fileutil.FileExists(absFile) {
		// If	the file exists, return the absolute path.
		return absFile, nil
	}

	// If the file does not exist, check if it exists in the DAGsDir.
	if ctx.opts.DAGsDir != "" {
		// If the file is not an absolute path, prepend the DAGsDir to the file name.
		file = filepath.Join(ctx.opts.DAGsDir, file)
	}

	// The file name can be specified without the extension.
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		file = fmt.Sprintf("%s.yaml", file)
	}

	return filepath.Abs(file)
}

// loadBaseConfigIfRequired loads the base config if needed, based on the given options.
func loadBaseConfigIfRequired(ctx BuildContext, baseConfig string) (*DAG, error) {
	if !ctx.opts.OnlyMetadata && baseConfig != "" {
		dag, err := LoadBaseConfig(ctx, baseConfig)
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
	// mergo does not override a value with zero value for a pointer.
	if typ == reflect.TypeOf(MailOn{}) {
		// We need to explicitly override the value for a pointer with a zero
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

// readYAMLFile reads the contents of the file into a map.
func readYAMLFile(file string) (cfg map[string]any, err error) {
	data, err := os.ReadFile(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", file, err)
	}

	return unmarshalData(data)
}

func readRawFile(file string) ([]byte, error) {
	data, err := os.ReadFile(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q: %v", file, err)
	}

	return data, nil
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
