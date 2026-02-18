package agent

import (
	"fmt"
	"path/filepath"
)

// CreateTools returns all registered agent tools, constructed with the given config.
func CreateTools(cfg ToolConfig) []*AgentTool {
	regs := RegisteredTools()
	tools := make([]*AgentTool, 0, len(regs))
	for _, reg := range regs {
		tools = append(tools, reg.Factory(cfg))
	}
	return tools
}

// GetToolByName finds a tool by name from the given slice, or nil if not found.
func GetToolByName(tools []*AgentTool, name string) *AgentTool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return tool
		}
	}
	return nil
}

// toolError creates a ToolOut marked as an error with a formatted message.
func toolError(format string, args ...any) ToolOut {
	return ToolOut{
		Content: fmt.Sprintf(format, args...),
		IsError: true,
	}
}

// resolvePath joins path with workingDir if path is relative and workingDir is set.
func resolvePath(path, workingDir string) string {
	if !filepath.IsAbs(path) && workingDir != "" {
		return filepath.Join(workingDir, path)
	}
	return path
}
