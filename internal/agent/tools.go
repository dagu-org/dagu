package agent

import (
	"fmt"
	"path/filepath"
)

// CreateTools returns all available tools for the agent including bash, read,
// patch, think, navigate, and schema tools.
func CreateTools() []*AgentTool {
	return []*AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewPatchTool(),
		NewThinkTool(),
		NewNavigateTool(),
		NewReadSchemaTool(),
	}
}

// GetToolByName finds a tool by name from the given slice.
// Returns nil if the tool is not found.
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

// resolvePath converts a relative path to absolute using the working directory.
// If the path is already absolute or workingDir is empty, returns path unchanged.
func resolvePath(path, workingDir string) string {
	if !filepath.IsAbs(path) && workingDir != "" {
		return filepath.Join(workingDir, path)
	}
	return path
}
