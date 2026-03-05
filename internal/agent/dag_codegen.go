package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "dag_codegen",
		Label:          "DAG Code Generation",
		Description:    "Generate DAG YAML definitions from structured step descriptions",
		DefaultEnabled: true,
		Factory:        func(cfg ToolConfig) *AgentTool { return NewDAGCodegenTool(cfg.DAGsDir) },
	})
}

// DAGCodegenInput is the input schema for the dag_codegen tool.
type DAGCodegenInput struct {
	Name  string           `json:"name"`
	Steps []DAGCodegenStep `json:"steps"`
	Tags  []string         `json:"tags"`
}

// DAGCodegenStep defines a single step in the generated DAG.
type DAGCodegenStep struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
	Dir     string   `json:"dir,omitempty"`
	Depends []string `json:"depends,omitempty"`
}

// NewDAGCodegenTool creates a new dag_codegen tool for generating DAG YAML files.
func NewDAGCodegenTool(dagsDir string) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "dag_codegen",
				Description: "Generate a DAG YAML file from structured step definitions. Creates a workflow with parallel/sequential steps based on dependency declarations.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Name for the generated DAG (used as filename)",
						},
						"steps": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{
										"type":        "string",
										"description": "Step name (must be unique within the DAG)",
									},
									"command": map[string]any{
										"type":        "string",
										"description": "Command to execute",
									},
									"args": map[string]any{
										"type":        "array",
										"items":       map[string]any{"type": "string"},
										"description": "Arguments to pass to the command",
									},
									"dir": map[string]any{
										"type":        "string",
										"description": "Working directory for this step",
									},
									"depends": map[string]any{
										"type":        "array",
										"items":       map[string]any{"type": "string"},
										"description": "Names of steps this step depends on",
									},
								},
								"required": []string{"name", "command"},
							},
							"description": "Step definitions for the DAG",
						},
						"tags": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "string"},
							"description": "Tags to apply to the generated DAG",
						},
					},
					"required": []string{"name", "steps"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			return dagCodegenRun(ctx, input, dagsDir)
		},
		Audit: &AuditInfo{
			Action:          "dag_codegen",
			DetailExtractor: ExtractFields("name"),
		},
	}
}

func dagCodegenRun(ctx ToolContext, input json.RawMessage, dagsDir string) ToolOut {
	if ctx.Role.IsSet() && !ctx.Role.CanWrite() {
		return toolError("Permission denied: dag_codegen requires write permission")
	}

	var args DAGCodegenInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Name == "" {
		return toolError("name is required")
	}
	if len(args.Steps) == 0 {
		return toolError("at least one step is required")
	}

	if err := validateSteps(args.Steps); err != nil {
		return toolError("%v", err)
	}

	yaml := buildDAGYAML(args)

	generatedDir := filepath.Join(dagsDir, ".generated")
	if err := os.MkdirAll(generatedDir, dirPermission); err != nil {
		return toolError("Failed to create .generated directory: %v", err)
	}

	filePath := filepath.Join(generatedDir, args.Name+".yaml")
	if err := os.WriteFile(filePath, []byte(yaml), filePermission); err != nil {
		return toolError("Failed to write DAG file: %v", err)
	}

	msg := fmt.Sprintf("Created DAG '%s' at .generated/%s.yaml with %d steps", args.Name, args.Name, len(args.Steps))
	if errs := validateGeneratedDAG(ctx.Context, filePath); len(errs) > 0 {
		msg += "\n\nDAG Validation Errors:\n- " + strings.Join(errs, "\n- ")
	}
	return ToolOut{Content: msg}
}

func validateSteps(steps []DAGCodegenStep) error {
	names := make(map[string]bool, len(steps))
	for _, s := range steps {
		if s.Name == "" {
			return fmt.Errorf("all steps must have a name")
		}
		if names[s.Name] {
			return fmt.Errorf("duplicate step name: %s", s.Name)
		}
		names[s.Name] = true
	}

	for _, s := range steps {
		for _, dep := range s.Depends {
			if !names[dep] {
				return fmt.Errorf("step %q depends on unknown step %q", s.Name, dep)
			}
		}
	}

	if err := detectCycle(steps); err != nil {
		return err
	}

	return nil
}

func detectCycle(steps []DAGCodegenStep) error {
	deps := make(map[string][]string, len(steps))
	for _, s := range steps {
		deps[s.Name] = s.Depends
	}

	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(steps))

	var visit func(name string) error
	visit = func(name string) error {
		if state[name] == visiting {
			return fmt.Errorf("dependency cycle detected involving step %q", name)
		}
		if state[name] == visited {
			return nil
		}
		state[name] = visiting
		for _, dep := range deps[name] {
			if err := visit(dep); err != nil {
				return err
			}
		}
		state[name] = visited
		return nil
	}

	for _, s := range steps {
		if err := visit(s.Name); err != nil {
			return err
		}
	}
	return nil
}

func buildDAGYAML(input DAGCodegenInput) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "name: %s\n", input.Name)
	buf.WriteString("type: graph\n")

	if len(input.Tags) > 0 {
		buf.WriteString("tags:\n")
		for _, tag := range input.Tags {
			fmt.Fprintf(&buf, "  - %s\n", tag)
		}
	}

	buf.WriteString("steps:\n")
	for _, s := range input.Steps {
		fmt.Fprintf(&buf, "  - name: %s\n", s.Name)
		fmt.Fprintf(&buf, "    command: %s\n", s.Command)
		if len(s.Args) > 0 {
			buf.WriteString("    args:\n")
			for _, arg := range s.Args {
				fmt.Fprintf(&buf, "      - %q\n", arg)
			}
		}
		if s.Dir != "" {
			fmt.Fprintf(&buf, "    dir: %s\n", s.Dir)
		}
		if len(s.Depends) > 0 {
			buf.WriteString("    depends:\n")
			for _, dep := range s.Depends {
				fmt.Fprintf(&buf, "      - %s\n", dep)
			}
		}
	}

	return buf.String()
}

func validateGeneratedDAG(ctx context.Context, path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{fmt.Sprintf("failed to read for validation: %v", err)}
	}

	_, err = spec.LoadYAML(ctx, data, spec.WithoutEval())
	if err != nil {
		var errList core.ErrorList
		if errors.As(err, &errList) {
			return errList.ToStringList()
		}
		return []string{err.Error()}
	}
	return nil
}
