// Package chat provides an executor for chat (LLM-based conversation) steps.
package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	llmpkg "github.com/dagu-org/dagu/internal/llm"
)

// ToolRegistry manages tool DAGs and converts them to LLM tool format.
type ToolRegistry struct {
	// tools maps tool name (from DAG.Name) to tool info
	tools map[string]*toolInfo
	// dagNames maps tool name to original DAG name (for lookup)
	dagNames map[string]string
}

// toolInfo contains the parsed tool information from a DAG.
type toolInfo struct {
	Name        string
	Description string
	DAG         *core.DAG
	Params      []toolParam
}

// toolParam represents a parsed parameter definition.
type toolParam struct {
	Name     string
	Type     string // "string", "integer", "boolean", "array", "object"
	Default  any
	Required bool
}

// NewToolRegistry creates a ToolRegistry by loading the specified DAGs.
// dagNames is a list of DAG names to load as tools.
// Tools are first searched in LocalDAGs (inline definitions with ---), then in the database.
func NewToolRegistry(ctx context.Context, dagNames []string) (*ToolRegistry, error) {
	if len(dagNames) == 0 {
		return nil, nil
	}

	rCtx := exec1.GetContext(ctx)

	registry := &ToolRegistry{
		tools:    make(map[string]*toolInfo),
		dagNames: make(map[string]string),
	}

	for _, dagName := range dagNames {
		var dag *core.DAG

		// First, check if it's a local DAG defined in the same file (using --- separator)
		// This follows the same pattern as SubDAGExecutor.NewSubDAGExecutor()
		if rCtx.DAG != nil && rCtx.DAG.LocalDAGs != nil {
			if localDAG, ok := rCtx.DAG.LocalDAGs[dagName]; ok {
				dag = localDAG
			}
		}

		// If not found locally, fall back to database lookup
		if dag == nil {
			if rCtx.DB == nil {
				return nil, fmt.Errorf("database not available in context and tool DAG %q not found in local DAGs", dagName)
			}
			var err error
			dag, err = rCtx.DB.GetDAG(ctx, dagName)
			if err != nil {
				return nil, fmt.Errorf("failed to load tool DAG %q: %w", dagName, err)
			}
		}

		// Use DAG.Name as the tool name (this is what the LLM will use)
		toolName := dag.Name
		if toolName == "" {
			toolName = dagName // Fallback to filename-based name
		}

		// Parse params from DefaultParams
		params, err := parseToolParams(dag.DefaultParams)
		if err != nil {
			return nil, fmt.Errorf("failed to parse params for tool DAG %q: %w", dagName, err)
		}

		info := &toolInfo{
			Name:        toolName,
			Description: dag.Description,
			DAG:         dag,
			Params:      params,
		}

		registry.tools[toolName] = info
		registry.dagNames[toolName] = dagName
	}

	return registry, nil
}

// ToLLMTools converts the registry to LLM tool format.
func (r *ToolRegistry) ToLLMTools() []llmpkg.Tool {
	if r == nil {
		return nil
	}

	tools := make([]llmpkg.Tool, 0, len(r.tools))
	for _, info := range r.tools {
		tools = append(tools, llmpkg.Tool{
			Type: "function",
			Function: llmpkg.ToolFunction{
				Name:        info.Name,
				Description: info.Description,
				Parameters:  buildJSONSchema(info.Params),
			},
		})
	}

	return tools
}

// GetDAGByToolName returns the DAG for a given tool name.
func (r *ToolRegistry) GetDAGByToolName(toolName string) (*core.DAG, bool) {
	if r == nil {
		return nil, false
	}
	info, ok := r.tools[toolName]
	if !ok {
		return nil, false
	}
	return info.DAG, true
}

// GetDAGName returns the original DAG name for a tool name.
func (r *ToolRegistry) GetDAGName(toolName string) (string, bool) {
	if r == nil {
		return "", false
	}
	name, ok := r.dagNames[toolName]
	return name, ok
}

// HasTools returns true if any tools are registered.
func (r *ToolRegistry) HasTools() bool {
	return r != nil && len(r.tools) > 0
}

// paramRegex matches "name" or "name=value" patterns in param strings.
var toolParamRegex = regexp.MustCompile(`([a-zA-Z_][a-zA-Z0-9_]*)(?:=(.*))?`)

// parseToolParams parses a DefaultParams string into toolParam slice.
// Format: "param1 param2=default2 param3=10"
func parseToolParams(defaultParams string) ([]toolParam, error) {
	if defaultParams == "" {
		return nil, nil
	}

	// Split by whitespace, but handle quoted values
	parts := splitParams(defaultParams)
	params := make([]toolParam, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		match := toolParamRegex.FindStringSubmatch(part)
		if match == nil {
			continue
		}

		name := match[1]
		defaultValue := ""
		if len(match) > 2 {
			defaultValue = match[2]
		}

		param := toolParam{
			Name:     name,
			Required: defaultValue == "",
		}

		if defaultValue != "" {
			param.Default, param.Type = inferTypeFromDefault(defaultValue)
		} else {
			param.Type = "string" // Default to string for required params
		}

		params = append(params, param)
	}

	return params, nil
}

// splitParams splits a param string by whitespace, respecting quotes.
func splitParams(s string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch {
		case (ch == '"' || ch == '\'') && !inQuote:
			inQuote = true
			quoteChar = ch
			current.WriteRune(ch)
		case ch == quoteChar && inQuote:
			inQuote = false
			quoteChar = 0
			current.WriteRune(ch)
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// inferTypeFromDefault infers the JSON Schema type from a default value string.
func inferTypeFromDefault(value string) (any, string) {
	// Remove surrounding quotes if present
	if (strings.HasPrefix(value, `"`) && strings.HasSuffix(value, `"`)) ||
		(strings.HasPrefix(value, `'`) && strings.HasSuffix(value, `'`)) {
		return value[1 : len(value)-1], "string"
	}

	// Try integer
	if i, err := strconv.ParseInt(value, 10, 64); err == nil {
		return i, "integer"
	}

	// Try float (maps to number in JSON Schema)
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return f, "number"
	}

	// Try boolean
	if value == "true" {
		return true, "boolean"
	}
	if value == "false" {
		return false, "boolean"
	}

	// Try JSON array
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		var arr []any
		if err := json.Unmarshal([]byte(value), &arr); err == nil {
			return arr, "array"
		}
	}

	// Try JSON object
	if strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}") {
		var obj map[string]any
		if err := json.Unmarshal([]byte(value), &obj); err == nil {
			return obj, "object"
		}
	}

	// Default to string
	return value, "string"
}

// buildJSONSchema builds a JSON Schema from tool parameters.
func buildJSONSchema(params []toolParam) map[string]any {
	properties := make(map[string]any)
	var required []string

	for _, param := range params {
		prop := map[string]any{
			"type":        param.Type,
			"description": fmt.Sprintf("%s parameter", param.Name),
		}

		if param.Default != nil {
			prop["default"] = param.Default
		}

		properties[param.Name] = prop

		if param.Required {
			required = append(required, param.Name)
		}
	}

	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}
