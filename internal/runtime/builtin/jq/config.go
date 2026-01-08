package jq

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"raw": {Type: "boolean", Description: "Output raw strings without JSON encoding (like jq -r)"},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("jq", configSchema)
}
