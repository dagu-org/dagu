package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	dirPermission  = 0o755
	filePermission = 0o644
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
func NewPatchTool() *AgentTool {
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
		Run: patchRun,
	}
}

func patchRun(ctx ToolContext, input json.RawMessage) ToolOut {
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
		return patchCreate(path, args.Content)
	case PatchOpReplace:
		return patchReplace(path, args.OldString, args.NewString)
	case PatchOpDelete:
		return patchDelete(path)
	default:
		return toolError("Unknown operation: %s. Use 'create', 'replace', or 'delete'.", args.Operation)
	}
}

func patchCreate(path, content string) ToolOut {
	if err := os.MkdirAll(filepath.Dir(path), dirPermission); err != nil {
		return toolError("Failed to create directory: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), filePermission); err != nil {
		return toolError("Failed to write file: %v", err)
	}

	return ToolOut{Content: fmt.Sprintf("Created %s (%d lines)", path, countLines(content))}
}

func patchReplace(path, oldString, newString string) ToolOut {
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

	switch count {
	case 0:
		return toolError("old_string not found in file. Make sure to include exact text including whitespace and indentation.")
	case 1:
		// Valid: exactly one match found
	default:
		return toolError("old_string found %d times in file. It must be unique. Include more context to make it unique.", count)
	}

	newContent := strings.Replace(contentStr, oldString, newString, 1)
	if err := os.WriteFile(path, []byte(newContent), filePermission); err != nil {
		return toolError("Failed to write file: %v", err)
	}

	return ToolOut{Content: fmt.Sprintf("Replaced %d lines with %d lines in %s", countLines(oldString), countLines(newString), path)}
}

func patchDelete(path string) ToolOut {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return toolError("File not found: %s", path)
	}

	if err := os.Remove(path); err != nil {
		return toolError("Failed to delete file: %v", err)
	}

	return ToolOut{Content: fmt.Sprintf("Deleted %s", path)}
}

func countLines(s string) int {
	return strings.Count(s, "\n") + 1
}
