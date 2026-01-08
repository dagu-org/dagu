package jq

import "github.com/dagu-org/dagu/internal/core"

// ConfigSchema defines the schema for jq executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	Raw bool `json:"raw,omitempty" jsonschema:"Output raw strings without JSON encoding (like jq -r)"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("jq")
}
