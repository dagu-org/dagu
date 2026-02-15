package iface

// MemoryContent holds loaded memory for system prompt injection.
type MemoryContent struct {
	GlobalMemory string // Contents of global MEMORY.md (truncated)
	DAGMemory    string // Contents of per-DAG MEMORY.md (truncated)
	DAGName      string // Name of the DAG (empty if no DAG context)
	MemoryDir    string // Root memory directory path
}
