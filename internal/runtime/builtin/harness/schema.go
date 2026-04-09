// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"github.com/dagucloud/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type:     "object",
	Required: []string{"provider"},
	Properties: map[string]*jsonschema.Schema{
		"provider": {Type: "string", Description: "Coding agent CLI provider (e.g., claude, codex, copilot, opencode, pi)"},
	},
	// All other keys are passed through as CLI flags.
	// No additional properties are restricted.
}

func init() {
	core.RegisterExecutorConfigSchema("harness", configSchema)
}
