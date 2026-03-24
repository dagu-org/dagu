// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package template

import (
	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"data":   {Type: "object", Description: "Template data variables accessible as {{ .key }} in the template"},
		"output": {Type: "string", Description: "File path to write the rendered output to. If empty, output is written to stdout."},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("template", configSchema)
}
