// Package schema provides the embedded DAG JSON schema for use across the codebase.
package schema

import _ "embed"

//go:embed dag.schema.json
var DAGSchemaJSON []byte

//go:embed config.schema.json
var ConfigSchemaJSON []byte
