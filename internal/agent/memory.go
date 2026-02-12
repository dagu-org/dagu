package agent

import "context"

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
}

// MemoryContent holds loaded memory for system prompt injection.
type MemoryContent struct {
	GlobalMemory string // Contents of global MEMORY.md (truncated)
	DAGMemory    string // Contents of per-DAG MEMORY.md (truncated)
	DAGName      string // Name of the DAG (empty if no DAG context)
	MemoryDir    string // Root memory directory path
}
