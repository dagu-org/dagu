package agent

import (
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

const (
	dirPermission  = 0o750
	filePermission = 0o600
)

// PatchOperation defines the type of patch operation.
type PatchOperation string

const (
	PatchOpCreate  PatchOperation = "create"
	PatchOpReplace PatchOperation = "replace"
	PatchOpDelete  PatchOperation = "delete"
)

// PatchToolInput is the input schema for the patch tool.
type PatchToolInput struct {
	Path      string         `json:"path"`
	Operation PatchOperation `json:"operation"`
	Content   string         `json:"content,omitempty"`    // For create operation
	OldString string         `json:"old_string,omitempty"` // For replace operation
	NewString string         `json:"new_string,omitempty"` // For replace operation
}

// NewPatchTool creates a new patch tool for file editing.
// The dagsDir parameter is used to auto-validate DAG files after write operations.
func NewPatchTool(dagsDir string) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "patch",
				Description: "Create, edit, or delete files. Use 'create' to write new files, 'replace' to edit existing files by replacing text, or 'delete' to remove files.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The path to the file (absolute or relative to working directory)",
						},
						"operation": map[string]any{
							"type":        "string",
							"enum":        []string{"create", "replace", "delete"},
							"description": "The operation to perform: 'create' (write new file), 'replace' (edit by replacing text), or 'delete' (remove file)",
						},
						"content": map[string]any{
							"type":        "string",
							"description": "For 'create': the full content to write to the file",
						},
						"old_string": map[string]any{
							"type":        "string",
							"description": "For 'replace': the exact text to find and replace (must be unique in the file)",
						},
						"new_string": map[string]any{
							"type":        "string",
							"description": "For 'replace': the text to replace old_string with",
						},
					},
					"required": []string{"path", "operation"},
				},
			},
		},
		Run: func(ctx ToolContext, input json.RawMessage) ToolOut {
			return patchRun(ctx, input, dagsDir)
		},
		Audit: &AuditInfo{
			Action:          "file_patch",
			DetailExtractor: ExtractFields("path", "operation"),
		},
	}
}

func patchRun(ctx ToolContext, input json.RawMessage, dagsDir string) ToolOut {
	var args PatchToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Path == "" {
		return toolError("Path is required")
	}

	path := resolvePath(args.Path, ctx.WorkingDir)

	switch args.Operation {
	case PatchOpCreate:
		return patchCreate(ctx.Context, path, args.Content, dagsDir)
	case PatchOpReplace:
		return patchReplace(ctx.Context, path, args.OldString, args.NewString, dagsDir)
	case PatchOpDelete:
		return patchDelete(path)
	default:
		return toolError("Unknown operation: %s. Use 'create', 'replace', or 'delete'.", args.Operation)
	}
}

func patchCreate(ctx context.Context, path, content, dagsDir string) ToolOut {
	if err := os.MkdirAll(filepath.Dir(path), dirPermission); err != nil {
		return toolError("Failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), filePermission); err != nil {
		return toolError("Failed to write file: %v", err)
	}

	msg := fmt.Sprintf("Created %s (%d lines)", path, countLines(content))
	if errs := validateIfDAGFile(ctx, path, dagsDir); len(errs) > 0 {
		msg += "\n\nDAG Validation Errors:\n- " + strings.Join(errs, "\n- ")
	}
	return ToolOut{Content: msg}
}

func patchReplace(ctx context.Context, path, oldString, newString, dagsDir string) ToolOut {
	if oldString == "" {
		return toolError("old_string is required for replace operation")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return toolError("File not found: %s. Use 'create' operation to create new files.", path)
		}
		return toolError("Failed to read file: %v", err)
	}

	contentStr := string(content)
	count := strings.Count(contentStr, oldString)

	if count == 0 {
		return toolError("old_string not found in file. Make sure to include exact text including whitespace and indentation.")
	}
	if count > 1 {
		return toolError("old_string found %d times in file. It must be unique. Include more context to make it unique.", count)
	}

	newContent := strings.Replace(contentStr, oldString, newString, 1)
	if err := os.WriteFile(path, []byte(newContent), filePermission); err != nil {
		return toolError("Failed to write file: %v", err)
	}

	msg := fmt.Sprintf("Replaced %d lines with %d lines in %s", countLines(oldString), countLines(newString), path)
	if errs := validateIfDAGFile(ctx, path, dagsDir); len(errs) > 0 {
		msg += "\n\nDAG Validation Errors:\n- " + strings.Join(errs, "\n- ")
	}
	return ToolOut{Content: msg}
}

func patchDelete(path string) ToolOut {
	err := os.Remove(path)
	if err != nil {
		if os.IsNotExist(err) {
			return toolError("File not found: %s", path)
		}
		return toolError("Failed to delete file: %v", err)
	}
	return ToolOut{Content: fmt.Sprintf("Deleted %s", path)}
}

func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}

// isDAGFile checks if the path is a YAML file within the DAGs directory.
// Uses filepath.Rel to prevent path containment bypass attacks (e.g., /dags-malicious/).
func isDAGFile(path, dagsDir string) bool {
	if dagsDir == "" || !strings.HasSuffix(path, ".yaml") {
		return false
	}
	cleanPath := filepath.Clean(path)
	cleanDAGsDir := filepath.Clean(dagsDir)

	// Use filepath.Rel to properly check path containment
	rel, err := filepath.Rel(cleanDAGsDir, cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return false
	}
	return true
}

// validateIfDAGFile validates the file if it's a DAG file, returning any validation errors.
func validateIfDAGFile(ctx context.Context, path, dagsDir string) []string {
	if !isDAGFile(path, dagsDir) {
		return nil
	}

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
