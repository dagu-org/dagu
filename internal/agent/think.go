package agent

import (
	"encoding/json"

	"github.com/dagu-org/dagu/internal/llm"
)

// ThinkToolInput is the input schema for the think tool.
type ThinkToolInput struct {
	Thought string `json:"thought"`
}

// NewThinkTool creates a new think tool for reasoning without action.
// This allows the LLM to think through complex problems step by step.
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

func thinkRun(ctx ToolContext, input json.RawMessage) ToolOut {
	var args ThinkToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolOut{
			Content: "Thought recorded.",
			IsError: false,
		}
	}

	// Simply acknowledge the thought - no action needed
	return ToolOut{
		Content: "Thought recorded. Continue with your plan.",
		IsError: false,
	}
}
