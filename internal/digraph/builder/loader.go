package builder

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

	"dario.cat/mergo"
	digraph "github.com/dagu-org/dagu/internal/digraph"
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
	name             string   // Name of the DAG.
	baseConfig       string   // Path to the base digraph.DAG configuration file.
	params           string   // Parameters to override default parameters in the DAG.
	paramsList       []string // List of parameters to override default parameters in the DAG.
	noEval           bool     // Flag to disable evaluation of dynamic fields.
	onlyMetadata     bool     // Flag to load only metadata without full digraph.DAG details.
	dagsDir          string   // Directory containing the digraph.DAG files.
	allowBuildErrors bool     // Flag to allow build errors.
}

// LoadOption is a function type for setting LoadOptions.
type LoadOption func(*LoadOptions)

// WithBaseConfig sets the base digraph.DAG configuration file.
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

// WithDAGsDir sets the directory containing the digraph.DAG files.
// This directory is used as the base path for resolving relative digraph.DAG file paths.
// When a digraph.DAG is loaded by name rather than absolute path, the system will look
// for the digraph.DAG file in this directory. If not specified, the current working
// directory is used as the default.
func WithDAGsDir(dagsDir string) LoadOption {
	return func(o *LoadOptions) {
		o.dagsDir = dagsDir
	}
}

// WithAllowBuildErrors allows build errors to be ignored during digraph.DAG loading.
// This is required for loading DAGs that may have errors in their definitions,
// such as missing steps or invalid configurations. When this option is set,
// the loader will return a digraph.DAG with the errors included in the DAG's `BuildErrors` field,
// and will not fail the loading process.
func WithAllowBuildErrors() LoadOption {
	return func(o *LoadOptions) {
		o.allowBuildErrors = true
	}
}

// Load loads a Directed Acyclic Graph (digraph.DAG) from a file path or name with the given options.
//
// The function handles different input formats:
//
// 1. Absolute paths:
//   - YAML files (.yaml/.yml): Processed with dynamic evaluation, including base configs,
//     parameters, and environment variables
//
// 2. Relative paths or filenames:
//   - Resolved against the DAGsDir specified in options
//   - If DAGsDir is not provided, the current working directory is used
//   - For YAML files, the extension is optional
//
// This approach provides a flexible way to load digraph.DAG definitions from multiple sources
// while supporting customization through the LoadOptions.
func Load(ctx context.Context, nameOrPath string, opts ...LoadOption) (*digraph.DAG, error) {
	if nameOrPath == "" {
		return nil, ErrNameOrPathRequired
	}
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	buildContext := BuildContext{
		ctx: ctx,
		opts: BuildOpts{
			Base:             options.baseConfig,
			Parameters:       options.params,
			ParametersList:   options.paramsList,
			OnlyMetadata:     options.onlyMetadata,
			NoEval:           options.noEval,
			Name:             options.name,
			DAGsDir:          options.dagsDir,
			AllowBuildErrors: options.allowBuildErrors,
		},
	}
	return loadDAG(buildContext, nameOrPath)
}

// LoadYAML loads the digraph.DAG from the given YAML data with the specified options.
func LoadYAML(ctx context.Context, data []byte, opts ...LoadOption) (*digraph.DAG, error) {
	var options LoadOptions
	for _, opt := range opts {
		opt(&options)
	}
	return LoadYAMLWithOpts(ctx, data, BuildOpts{
		Base:             options.baseConfig,
		Parameters:       options.params,
		ParametersList:   options.paramsList,
		OnlyMetadata:     options.onlyMetadata,
		NoEval:           options.noEval,
		Name:             options.name,
		DAGsDir:          options.dagsDir,
		AllowBuildErrors: options.allowBuildErrors,
	})
}

// LoadYAMLWithOpts loads the digraph.DAG configuration from YAML data.
func LoadYAMLWithOpts(ctx context.Context, data []byte, opts BuildOpts) (*digraph.DAG, error) {
	raw, err := unmarshalData(data)
	if err != nil {
		if opts.AllowBuildErrors {
			// Return a minimal digraph.DAG with the error recorded
			return &digraph.DAG{
				Name:        opts.Name,
				BuildErrors: []error{err},
			}, nil
		}
		return nil, digraph.ErrorList{err}
	}

	def, err := decode(raw)
	if err != nil {
		if opts.AllowBuildErrors {
			// Return a minimal digraph.DAG with the error recorded
			return &digraph.DAG{
				Name:        opts.Name,
				BuildErrors: []error{err},
			}, nil
		}
		return nil, digraph.ErrorList{err}
	}

	return build(BuildContext{ctx: ctx, opts: opts}, def)
}

// LoadBaseConfig loads the global configuration from the given file.
// The global configuration can be overridden by the digraph.DAG configuration.
func LoadBaseConfig(ctx BuildContext, file string) (*digraph.DAG, error) {
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
		return nil, digraph.ErrorList{err}
	}

	ctx = ctx.WithOpts(BuildOpts{NoEval: ctx.opts.NoEval}).WithFile(file)
	dag, err := build(ctx, def)

	if err != nil {
		return nil, digraph.ErrorList{err}
	}
	return dag, nil
}

// loadDAG loads the digraph.DAG from the given file.
func loadDAG(ctx BuildContext, nameOrPath string) (*digraph.DAG, error) {
	filePath, err := resolveYamlFilePath(ctx, nameOrPath)
	if err != nil {
		return nil, err
	}

	ctx = ctx.WithFile(filePath)

	// Load base config definition if specified
	var baseDef *definition
	if !ctx.opts.OnlyMetadata && ctx.opts.Base != "" && fileutil.FileExists(ctx.opts.Base) {
		raw, err := readYAMLFile(ctx.opts.Base)
		if err != nil {
			return nil, fmt.Errorf("failed to read base config: %w", err)
		}
		baseDef, err = decode(raw)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base config: %w", err)
		}
	}

	// Load all DAGs from the file
	dags, err := loadDAGsFromFile(ctx, filePath, baseDef)
	if err != nil {
		if ctx.opts.AllowBuildErrors {
			// Return a minimal digraph.DAG with the error recorded
			dag := &digraph.DAG{
				Name:        defaultName(filePath),
				Location:    filePath,
				BuildErrors: []error{err},
			}
			return dag, nil
		}
		return nil, err
	}

	if len(dags) == 0 {
		return nil, fmt.Errorf("no DAGs found in file %q", filePath)
	}

	// Get the main digraph.DAG (first one)
	mainDAG := dags[0]

	// If there are child DAGs, add them to the main digraph.DAG
	if len(dags) > 1 {
		mainDAG.LocalDAGs = make(map[string]*digraph.DAG)
		for i := 1; i < len(dags); i++ {
			childDAG := dags[i]
			if childDAG.Name == "" {
				return nil, fmt.Errorf("child digraph.DAG at index %d must have a name", i)
			}
			mainDAG.LocalDAGs[childDAG.Name] = childDAG
		}
	}

	digraph.InitializeDefaults(mainDAG)

	return mainDAG, nil
}

// loadDAGsFromFile loads all DAGs from a multi-document YAML file
func loadDAGsFromFile(ctx BuildContext, filePath string, baseDef *definition) ([]*digraph.DAG, error) {
	// Open the file
	f, err := os.Open(filePath) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to open file %q: %w", filePath, err)
	}
	defer func() { _ = f.Close() }()

	// Read data from the file
	dat, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %q", filePath)
	}

	var dags []*digraph.DAG
	decoder := yaml.NewDecoder(bytes.NewReader(dat))

	// Read all documents from the file
	docIndex := 0
	for {
		var doc map[string]any
		err := decoder.Decode(&doc)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Note: The YAML decoder has limitations with empty documents
			// and may return errors for them. We skip and continue.
			return nil, fmt.Errorf("failed to decode document %d: %w", docIndex, err)
		}

		// Skip empty documents
		if len(doc) == 0 {
			docIndex++
			continue
		}

		// Update the context with the current document index
		ctx.index = docIndex

		// Decode the document into definition
		spec, err := decode(doc)
		if err != nil {
			return nil, fmt.Errorf("failed to decode document %d: %w", docIndex, err)
		}

		// Build a fresh base digraph.DAG from base definition if provided
		var dest *digraph.DAG
		if baseDef != nil {
			// Build a new base digraph.DAG for this document
			buildCtx := ctx
			// Don't parse parameters for the base digraph.DAG
			buildCtx.opts.Parameters = ""
			buildCtx.opts.ParametersList = nil
			// Build the base digraph.DAG
			baseDAG, err := build(buildCtx, baseDef)
			if err != nil {
				return nil, fmt.Errorf("failed to build base digraph.DAG for document %d: %w", docIndex, err)
			}
			dest = baseDAG
		} else {
			dest = new(digraph.DAG)
		}

		// Build the digraph.DAG from the current document
		dag, err := build(ctx, spec)
		if err != nil {
			return nil, fmt.Errorf("failed to build digraph.DAG in document %d: %w", docIndex, err)
		}

		// Merge the current digraph.DAG into the base digraph.DAG
		if err := merge(dest, dag); err != nil {
			return nil, fmt.Errorf("failed to merge digraph.DAG in document %d: %w", docIndex, err)
		}

		// Set the location for the digraph.DAG
		dest.Location = filePath

		if docIndex == 0 {
			// If this is the first document, set the entire digraph.DAG
			dest.YamlData = dat
		} else {
			// Marshal the document back to YAML to preserve original data
			yamlData, err := yaml.Marshal(doc)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal digraph.DAG in document %d: %w", docIndex, err)
			}
			dest.YamlData = yamlData
		}

		dags = append(dags, dest)
		docIndex++
	}

	// Validate unique names in multi-DAG files
	if len(dags) > 1 {
		names := make(map[string]bool)
		for i, dag := range dags {
			// Skip validation for the first digraph.DAG as it's the main digraph.DAG
			if i == 0 {
				continue
			}
			if dag.Name == "" {
				return nil, fmt.Errorf("DAG at index %d must have a name in multi-DAG file", i)
			}
			if names[dag.Name] {
				return nil, fmt.Errorf("duplicate DAG name %q found", dag.Name)
			}
			names[dag.Name] = true
		}
	}

	return dags, nil
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

	file = strings.TrimSpace(file) // Remove leading and trailing whitespace

	if filepath.IsAbs(file) {
		// If the file is an absolute path, return it as is.
		return file, nil
	}

	// Replace '~' with the user's home directory if present.
	if strings.HasPrefix(file, "~") {
		if homeDir, err := os.UserHomeDir(); err == nil {
			file = strings.Replace(file, "~", homeDir, 1)
		}
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

type mergeTransformer struct{}

var _ mergo.Transformers = (*mergeTransformer)(nil)

func (*mergeTransformer) Transformer(
	typ reflect.Type,
) func(dst, src reflect.Value) error {
	// mergo does not override a value with zero value for a pointer.
	if typ == reflect.TypeOf(digraph.MailOn{}) {
		// We need to explicitly override the value for a pointer with a zero
		// value.
		return func(dst, src reflect.Value) error {
			if dst.CanSet() {
				dst.Set(src)
			}

			return nil
		}
	}

	// Handle []string fields (like Env) by appending instead of replacing
	if typ == reflect.TypeOf([]string{}) {
		return func(dst, src reflect.Value) error {
			if !dst.CanSet() || src.Len() == 0 {
				return nil
			}

			// Append src values to dst
			result := reflect.AppendSlice(dst, src)
			dst.Set(result)

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

// merge merges the source digraph.DAG into the destination DAG.
func merge(dst, src *digraph.DAG) error {
	return mergo.Merge(dst, src, mergo.WithOverride,
		mergo.WithTransformers(&mergeTransformer{}))
}
