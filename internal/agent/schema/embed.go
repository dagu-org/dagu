package schema

import (
	cmnschema "github.com/dagu-org/dagu/internal/cmn/schema"
)

func init() {
	if err := DefaultRegistry.Register("dag", cmnschema.DAGSchemaJSON); err != nil {
		// Schema is embedded at compile time - if it fails to parse,
		// the binary is misconfigured and should fail fast.
		panic("failed to register dag schema: " + err.Error())
	}
}
