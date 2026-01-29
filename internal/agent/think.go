package agent

import (
	"encoding/json"

	"github.com/dagu-org/dagu/internal/llm"
)

// NewThinkTool creates a think tool for step-by-step reasoning without action.
func NewThinkTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "think",
				Description: "Use this tool to think through a problem step by step. The thought will be recorded but no action will be taken. Use this to plan complex operations before executing them.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"thought": map[string]any{
							"type":        "string",
							"description": "Your reasoning or thought process",
						},
					},
					"required": []string{"thought"},
				},
			},
		},
		Run: thinkRun,
	}
}

// thinkRun acknowledges the thought without performing any action.
func thinkRun(_ ToolContext, _ json.RawMessage) ToolOut {
	return ToolOut{Content: "Thought recorded. Continue with your plan."}
}
