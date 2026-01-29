package agent

import (
	"encoding/json"

	"github.com/dagu-org/dagu/internal/llm"
)

// NavigateToolInput defines the input parameters for the navigate tool.
type NavigateToolInput struct {
	Path string `json:"path"`
}

// NewNavigateTool creates a new navigate tool for UI navigation.
func NewNavigateTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "navigate",
				Description: "Navigate the user to a specific page in the Dagu UI. Use this after creating or modifying a DAG to show it to the user. Common paths: '/dags/<dag-name>' for DAG details, '/dags/<dag-name>/history' for run history.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The path to navigate to (e.g., '/dags/my-dag', '/dags/my-dag/history')",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Run: navigateRun,
	}
}

func navigateRun(ctx ToolContext, input json.RawMessage) ToolOut {
	var args NavigateToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Path == "" {
		return toolError("Path is required")
	}

	if ctx.EmitUIAction != nil {
		ctx.EmitUIAction(UIAction{
			Type: "navigate",
			Path: args.Path,
		})
	}

	return ToolOut{Content: "Navigating user to " + args.Path}
}
