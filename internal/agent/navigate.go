package agent

import (
	"encoding/json"
	"fmt"

	"github.com/dagu-org/dagu/internal/llm"
)

// NavigateToolInput is the input schema for the navigate tool.
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
				Description: "Navigate the user to a specific page in the Dagu UI. Only use this when the user explicitly asks to navigate or view a page. Available paths: '/dags' for DAG list, '/dags/<dag-name>' for DAG details, '/dags/<dag-name>/<tab>' for specific tab (spec, history), '/dag-runs' for all DAG runs, '/dag-runs/<dag-name>/<run-id>' for specific run details, '/queues' for queues. Admin pages: '/system-status', '/users', '/api-keys', '/webhooks', '/terminal', '/audit-logs', '/git-sync', '/agent-settings'.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The path to navigate to (e.g., '/dags/my-dag', '/dags/my-dag/spec', '/dag-runs')",
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

	return ToolOut{Content: fmt.Sprintf("Navigating user to %s", args.Path)}
}
