package agent

import "slices"

// ToolRegistration contains metadata and a factory for a single tool.
// Each tool registers itself via init() so the registry is the single source of truth.
type ToolRegistration struct {
	// Name is the unique tool identifier (e.g. "bash", "read").
	Name string
	// Label is the human-readable display name for the settings UI.
	Label string
	// Description is a short description for the settings UI.
	Description string
	// DefaultEnabled indicates whether the tool is enabled by default in policy.
	DefaultEnabled bool
	// Factory creates an instance of the tool with the given config.
	Factory func(ToolConfig) *AgentTool
}

// ToolConfig provides runtime configuration for tool construction.
type ToolConfig struct {
	// DAGsDir is the directory containing DAG definition files.
	DAGsDir string
}

// toolRegistry holds all registered tools. Populated by init() calls.
var toolRegistry []ToolRegistration

// RegisterTool adds a tool to the global registry.
// Must be called from init() functions only — not safe for concurrent use.
func RegisterTool(reg ToolRegistration) {
	toolRegistry = append(toolRegistry, reg)
}

// RegisteredTools returns all registered tool metadata.
func RegisteredTools() []ToolRegistration {
	return toolRegistry
}

// RegisteredToolNames returns sorted names of all registered tools.
func RegisteredToolNames() []string {
	names := make([]string, 0, len(toolRegistry))
	for _, reg := range toolRegistry {
		names = append(names, reg.Name)
	}
	slices.Sort(names)
	return names
}

// IsRegisteredTool returns true if the named tool is in the registry.
func IsRegisteredTool(name string) bool {
	for _, reg := range toolRegistry {
		if reg.Name == name {
			return true
		}
	}
	return false
}
