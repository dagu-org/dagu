package http

import "github.com/dagu-org/dagu/internal/core"

// ConfigSchema defines the schema for http executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	Timeout       *int              `json:"timeout,omitempty" jsonschema:"Request timeout in seconds"`
	Headers       map[string]string `json:"headers,omitempty" jsonschema:"HTTP headers to send"`
	Query         map[string]string `json:"query,omitempty" jsonschema:"Query parameters"`
	Body          string            `json:"body,omitempty" jsonschema:"Request body content"`
	Silent        bool              `json:"silent,omitempty" jsonschema:"Suppress headers/status output on success"`
	Debug         bool              `json:"debug,omitempty" jsonschema:"Enable debug mode"`
	JSON          bool              `json:"json,omitempty" jsonschema:"Format output as JSON"`
	SkipTLSVerify bool              `json:"skipTLSVerify,omitempty" jsonschema:"Skip TLS certificate verification"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("http")
}
