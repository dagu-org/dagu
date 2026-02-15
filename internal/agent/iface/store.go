package iface

import "context"

// ConfigStore provides access to agent configuration.
// All implementations must be safe for concurrent use.
type ConfigStore interface {
	// Load reads the agent configuration.
	Load(ctx context.Context) (*Config, error)
	// Save writes the agent configuration.
	Save(ctx context.Context, cfg *Config) error
	// IsEnabled returns whether the agent feature is enabled.
	IsEnabled(ctx context.Context) bool
}

// ModelStore defines the interface for model configuration persistence.
// All implementations must be safe for concurrent use.
type ModelStore interface {
	Create(ctx context.Context, model *ModelConfig) error
	GetByID(ctx context.Context, id string) (*ModelConfig, error)
	List(ctx context.Context) ([]*ModelConfig, error)
	Update(ctx context.Context, model *ModelConfig) error
	Delete(ctx context.Context, id string) error
}

// MemoryStore provides access to agent memory files.
// All implementations must be safe for concurrent use.
type MemoryStore interface {
	// LoadGlobalMemory reads the global MEMORY.md, truncated to maxLines.
	LoadGlobalMemory(ctx context.Context) (string, error)

	// LoadDAGMemory reads the MEMORY.md for a specific DAG, truncated to maxLines.
	LoadDAGMemory(ctx context.Context, dagName string) (string, error)

	// SaveGlobalMemory writes content to the global MEMORY.md.
	SaveGlobalMemory(ctx context.Context, content string) error

	// SaveDAGMemory writes content to a DAG-specific MEMORY.md.
	SaveDAGMemory(ctx context.Context, dagName string, content string) error

	// MemoryDir returns the root memory directory path.
	MemoryDir() string

	// ListDAGMemories returns the names of all DAGs that have memory files.
	ListDAGMemories(ctx context.Context) ([]string, error)

	// DeleteGlobalMemory removes the global MEMORY.md file.
	DeleteGlobalMemory(ctx context.Context) error

	// DeleteDAGMemory removes a DAG-specific MEMORY.md file.
	DeleteDAGMemory(ctx context.Context, dagName string) error
}
