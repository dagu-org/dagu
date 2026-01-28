package agent

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
