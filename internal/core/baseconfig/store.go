package baseconfig

import "context"

// Store provides access to the base DAG configuration.
// The base config is a YAML file whose values are inherited by all DAGs.
// All implementations must be safe for concurrent use.
type Store interface {
	// GetSpec returns the raw YAML content of the base configuration.
	// Returns an empty string if the file does not exist.
	GetSpec(ctx context.Context) (string, error)
	// UpdateSpec writes the given YAML content to the base configuration file.
	UpdateSpec(ctx context.Context, spec []byte) error
}
