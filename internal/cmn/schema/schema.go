// Package schema provides embedded JSON schemas for use across the codebase.
package schema

import _ "embed"

//go:embed dag.schema.json
var DAGSchemaJSON []byte

//go:embed config.schema.json
var ConfigSchemaJSON []byte
