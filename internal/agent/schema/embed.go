package schema

import (
	_ "embed"
	"log"
)

//go:embed dag.schema.json
var dagSchemaJSON []byte

func init() {
	if err := DefaultRegistry.Register("dag", dagSchemaJSON); err != nil {
		log.Printf("Failed to register dag schema: %v", err)
	}
}
