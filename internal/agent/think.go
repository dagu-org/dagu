package agent

import (
	"encoding/json"

	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "think",
		Label:          "Think",
		Description:    "Step-by-step reasoning",
		DefaultEnabled: true,
		Factory:        func(_ ToolConfig) *AgentTool { return NewThinkTool() },
	})
}

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
// The thought content is automatically recorded in session history by the caller
// (agent loop), so the tool itself only needs to return an acknowledgment.
func thinkRun(_ ToolContext, _ json.RawMessage) ToolOut {
	return ToolOut{Content: "Thought recorded. Continue with your plan."}
}
