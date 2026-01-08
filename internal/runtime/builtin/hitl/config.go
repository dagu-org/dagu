package hitl

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// Config holds the configuration for a HITL step.
type Config struct {
	// Prompt is the message displayed to the approver.
	Prompt string `mapstructure:"prompt"`
	// Input is the list of expected input field names from the approver.
	Input []string `mapstructure:"input"`
	// Required is the subset of Input fields that must be provided.
	Required []string `mapstructure:"required"`
}

func decodeConfig(dat map[string]any, cfg *Config) error {
	md, _ := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		WeaklyTypedInput: true,
		ErrorUnused:      false,
		Result:           cfg,
	})
	return md.Decode(dat)
}

// ConfigSchema defines the schema for hitl executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	Prompt   string   `json:"prompt,omitempty" jsonschema:"Message displayed to the approver"`
	Input    []string `json:"input,omitempty" jsonschema:"List of expected input field names"`
	Required []string `json:"required,omitempty" jsonschema:"Subset of Input fields that must be provided"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("hitl")
}
