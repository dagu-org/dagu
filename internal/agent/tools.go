package agent

// CreateTools creates all available tools for the agent.
// Returns a slice of AgentTool pointers that can be used by the Loop.
func CreateTools() []*AgentTool {
	return []*AgentTool{
		NewBashTool(),
		NewReadTool(),
		NewPatchTool(),
		NewThinkTool(),
		NewNavigateTool(),
	}
}

// GetToolByName returns a tool by its name, or nil if not found.
func GetToolByName(tools []*AgentTool, name string) *AgentTool {
	for _, t := range tools {
		if t.Function.Name == name {
			return t
		}
	}
	return nil
}
