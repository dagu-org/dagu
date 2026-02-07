package schema

import (
	cmnschema "github.com/dagu-org/dagu/internal/cmn/schema"
)

func init() {
	// Schemas are embedded at compile time. If parsing fails, the binary
	// is misconfigured and should fail fast.
	mustRegister("dag", cmnschema.DAGSchemaJSON)
	mustRegister("config", cmnschema.ConfigSchemaJSON)
}

func mustRegister(name string, data []byte) {
	if err := DefaultRegistry.Register(name, data); err != nil {
		panic("failed to register " + name + " schema: " + err.Error())
	}
}
