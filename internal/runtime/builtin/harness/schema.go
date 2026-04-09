// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"github.com/dagucloud/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"provider":    {Type: "string", Description: "Built-in provider name (claude, codex, copilot, opencode, pi)"},
		"binary":      {Type: "string", Description: "Custom CLI binary name (alternative to provider)"},
		"prompt_args": {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Base args for passing the prompt to a custom binary (default: [\"-p\"])"},
		"fallback": {
			Type: "array",
			Items: &jsonschema.Schema{
				Type: "object",
			},
			Description: "Ordered alternative provider configs tried after the primary config fails",
		},
	},
	// Either provider or binary is required (validated in Go).
	// All other keys are passed through as CLI flags.
}

func init() {
	core.RegisterExecutorConfigSchema("harness", configSchema)
}
