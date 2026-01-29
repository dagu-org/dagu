package schema

import (
	_ "embed"
)

//go:embed dag.schema.json
var dagSchemaJSON []byte

func init() {
	if err := DefaultRegistry.Register("dag", dagSchemaJSON); err != nil {
		// Schema is embedded at compile time - if it fails to parse,
		// the binary is misconfigured and should fail fast.
		panic("failed to register dag schema: " + err.Error())
	}
}
