package archive

import (
	"fmt"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// Config contains runtime options for the archive executor.
type Config struct {
	Source           string   `mapstructure:"source"`
	Destination      string   `mapstructure:"destination"`
	Format           string   `mapstructure:"format"`
	CompressionLevel int      `mapstructure:"compressionLevel"`
	Password         string   `mapstructure:"password"`
	Overwrite        bool     `mapstructure:"overwrite"`
	PreservePaths    bool     `mapstructure:"preservePaths"`
	StripComponents  int      `mapstructure:"stripComponents"`
	Include          []string `mapstructure:"include"`
	Exclude          []string `mapstructure:"exclude"`
	DryRun           bool     `mapstructure:"dryRun"`
	Verbose          bool     `mapstructure:"verbose"`
	FollowSymlinks   bool     `mapstructure:"followSymlinks"`
	VerifyIntegrity  bool     `mapstructure:"verifyIntegrity"`
	ContinueOnError  bool     `mapstructure:"continueOnError"`
}

func defaultConfig() Config {
	return Config{
		PreservePaths:    true,
		CompressionLevel: -1,
	}
}

func decodeConfig(raw map[string]any, cfg *Config) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		TagName:          "mapstructure",
	})
	if err != nil {
		return err
	}
	return decoder.Decode(raw)
}

func validateConfig(operation string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("%w: missing configuration", ErrConfig)
	}
	switch operation {
	case opExtract, opList:
		if cfg.Source == "" {
			return fmt.Errorf("%w: source is required for %s", ErrConfig, operation)
		}
	case opCreate:
		if cfg.Source == "" {
			return fmt.Errorf("%w: source is required for %s", ErrConfig, operation)
		}
		if cfg.Destination == "" && !cfg.DryRun {
			return fmt.Errorf("%w: destination is required for %s", ErrConfig, operation)
		}
	}

	if cfg.StripComponents < 0 {
		return fmt.Errorf("%w: stripComponents must be >= 0", ErrConfig)
	}

	if cfg.Password != "" && operation != opExtract && operation != opList {
		return fmt.Errorf("%w: password is only supported for extract/list operations", ErrConfig)
	}

	for _, pattern := range append(cfg.Include, cfg.Exclude...) {
		if pattern == "" {
			continue
		}
		if !doublestar.ValidatePattern(pattern) {
			return fmt.Errorf("%w: invalid glob pattern %q", ErrConfig, pattern)
		}
	}

	return nil
}

// ConfigSchema defines the schema for archive executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	Source           string   `json:"source" jsonschema:"File or directory to archive/extract"`
	Destination      string   `json:"destination,omitempty" jsonschema:"Archive file path (required for create)"`
	Format           string   `json:"format,omitempty" jsonschema:"Archive format (zip, tar, tar.gz, etc.)"`
	CompressionLevel *int     `json:"compressionLevel,omitempty" jsonschema:"Compression level (-1 for default)"`
	Password         string   `json:"password,omitempty" jsonschema:"Password for extract/list only"`
	Overwrite        bool     `json:"overwrite,omitempty" jsonschema:"Overwrite existing files"`
	PreservePaths    *bool    `json:"preservePaths,omitempty" jsonschema:"Preserve directory structure (default: true)"`
	StripComponents  *int     `json:"stripComponents,omitempty" jsonschema:"Strip leading path components (must be >= 0)"`
	Include          []string `json:"include,omitempty" jsonschema:"Glob patterns to include"`
	Exclude          []string `json:"exclude,omitempty" jsonschema:"Glob patterns to exclude"`
	DryRun           bool     `json:"dryRun,omitempty" jsonschema:"Simulate without making changes"`
	Verbose          bool     `json:"verbose,omitempty" jsonschema:"Enable verbose output"`
	FollowSymlinks   bool     `json:"followSymlinks,omitempty" jsonschema:"Follow symbolic links"`
	VerifyIntegrity  bool     `json:"verifyIntegrity,omitempty" jsonschema:"Verify archive integrity"`
	ContinueOnError  bool     `json:"continueOnError,omitempty" jsonschema:"Continue on errors"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("archive")
}
