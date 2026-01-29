package agent

import (
	"fmt"
	"path/filepath"
)

// CreateTools creates all available tools for the agent.
func CreateTools() []*AgentTool {
	return []*AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewPatchTool(),
		NewThinkTool(),
		NewNavigateTool(),
		NewDagReferenceTool(),
	}
}

// GetToolByName returns a tool by its name, or nil if not found.
func GetToolByName(tools []*AgentTool, name string) *AgentTool {
	for _, tool := range tools {
		if tool.Function.Name == name {
			return tool
		}
	}
	return nil
}

// toolError creates a ToolOut with an error message.
func toolError(format string, args ...any) ToolOut {
	return ToolOut{
		Content: fmt.Sprintf(format, args...),
		IsError: true,
	}
}

// resolvePath resolves a relative path against a working directory.
func resolvePath(path, workingDir string) string {
	if !filepath.IsAbs(path) && workingDir != "" {
		return filepath.Join(workingDir, path)
	}
	return path
}
