package hitl

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
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

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"prompt":   {Type: "string", Description: "Message displayed to the approver"},
		"input":    {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "List of expected input field names"},
		"required": {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Subset of Input fields that must be provided"},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("hitl", configSchema)
}
