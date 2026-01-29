package agent

import (
	"encoding/json"
	"strings"

	"github.com/dagu-org/dagu/internal/agent/schema"
	"github.com/dagu-org/dagu/internal/llm"
)

// ReadSchemaInput is the input schema for the read_schema tool.
type ReadSchemaInput struct {
	Schema string `json:"schema"`
	Path   string `json:"path"`
}

const readSchemaDescription = "Navigate JSON schema documentation to understand DAG YAML structure. " +
	"Use this before creating or editing DAG files to ensure correct syntax. " +
	"Call with empty path to see root-level fields, or specify a path to drill down (e.g., 'steps.container')."

// NewReadSchemaTool creates a new tool for navigating JSON schemas.
func NewReadSchemaTool() *AgentTool {
	available := schema.DefaultRegistry.AvailableSchemas()

	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "read_schema",
				Description: readSchemaDescription,
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"schema": map[string]any{
							"type":        "string",
							"description": "Schema to navigate: " + strings.Join(available, ", "),
							"enum":        available,
						},
						"path": map[string]any{
							"type":        "string",
							"description": "Dot-separated path to navigate (e.g., 'steps', 'steps.container', 'handlerOn.success'). Empty for root level.",
						},
					},
					"required": []string{"schema"},
				},
			},
		},
		Run: readSchemaRun,
	}
}

func readSchemaRun(_ ToolContext, input json.RawMessage) ToolOut {
	var args ReadSchemaInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Schema == "" {
		return toolError("Schema name is required. Available: %s", strings.Join(schema.DefaultRegistry.AvailableSchemas(), ", "))
	}

	result, err := schema.DefaultRegistry.Navigate(args.Schema, args.Path)
	if err != nil {
		return toolError("%v", err)
	}

	return ToolOut{Content: result}
}
