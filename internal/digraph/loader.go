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
	"github.com/go-viper/mapstructure/v2"
	"github.com/imdario/mergo"

	"gopkg.in/yaml.v2"
)

// LoadOptions contains options for loading a DAG.
type LoadOptions struct {
	baseConfig   string   // Path to the base DAG configuration file.
	params       string   // Parameters to override default parameters in the DAG.
	paramsList   []string // List of parameters to override default parameters in the DAG.
	noEval       bool     // Flag to disable evaluation of dynamic fields.
	onlyMetadata bool     // Flag to load only metadata without full DAG details.
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

// Load loads the DAG from the given file with the specified options.
func Load(ctx context.Context, dag string, opts ...LoadOption) (*DAG, error) {
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	buildContext := BuildContext{
		ctx: ctx,
		opts: buildOpts{
			base:           options.baseConfig,
			parameters:     options.params,
			parametersList: options.paramsList,
			onlyMetadata:   options.onlyMetadata,
			noEval:         options.noEval,
		},
	}
	return loadDAG(buildContext, dag)
}

// LoadYAML loads the DAG from the given YAML data with the specified options.
func LoadYAML(ctx context.Context, data []byte, opts ...LoadOption) (*DAG, error) {
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	return loadYAML(ctx, data, buildOpts{
		base:           options.baseConfig,
		parameters:     options.params,
		parametersList: options.paramsList,
		onlyMetadata:   options.onlyMetadata,
		noEval:         options.noEval,
	})
}

// loadYAML loads the DAG configuration from YAML data.
func loadYAML(ctx context.Context, data []byte, opts buildOpts) (*DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		return nil, err
	}

	def, err := decode(raw)
	if err != nil {
		return nil, err
	}

	return build(BuildContext{ctx: ctx, opts: opts}, def)
}

// loadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the DAG configuration.
func loadBaseConfig(ctx BuildContext, file string) (*DAG, error) {
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

	ctx = ctx.WithOpts(buildOpts{noEval: ctx.opts.noEval}).WithFile(file)
	return build(ctx, def)
}

// loadDAG loads the DAG from the given file.
func loadDAG(ctx BuildContext, dag string) (*DAG, error) {
	filePath, err := resolveYamlFilePath(dag)
	if err != nil {
		return nil, err
	}

	ctx = ctx.WithFile(filePath)

	dest, err := loadBaseConfigIfRequired(ctx, ctx.opts.base)
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

	target, err := build(ctx, spec)
	if err != nil {
		return nil, err
	}

	// Merge the target DAG into the dest DAG.
	err = merge(dest, target)
	if err != nil {
		return nil, err
	}

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

// resolveYamlFilePath resolves the YAML file path.
// If the file name does not have an extension, it appends ".yaml".
func resolveYamlFilePath(file string) (string, error) {
	if file == "" {
		return "", errors.New("file path is required")
	}

	// The file name can be specified without the extension.
	if !strings.HasSuffix(file, ".yaml") && !strings.HasSuffix(file, ".yml") {
		file = fmt.Sprintf("%s.yaml", file)
	}

	return filepath.Abs(file)
}

// loadBaseConfigIfRequired loads the base config if needed, based on the given options.
func loadBaseConfigIfRequired(ctx BuildContext, baseConfig string) (*DAG, error) {
	if !ctx.opts.onlyMetadata && baseConfig != "" {
		dag, err := loadBaseConfig(ctx, baseConfig)
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
