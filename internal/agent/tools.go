package agent

import (
	"fmt"
	"path/filepath"
)

// CreateTools returns all available tools for the agent including bash, read,
// patch, think, navigate, schema, and ask_user tools.
// The dagsDir parameter is used by the patch tool for DAG file validation.
func CreateTools(dagsDir string) []*AgentTool {
	return []*AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewPatchTool(dagsDir),
		NewThinkTool(),
		NewNavigateTool(),
		NewReadSchemaTool(),
		NewAskUserTool(),
		NewWebSearchTool(),
	}
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
