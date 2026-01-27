package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

// PatchOperation defines the type of patch operation.
type PatchOperation string

const (
	// PatchOpCreate creates a new file with the given content.
	PatchOpCreate PatchOperation = "create"
	// PatchOpReplace replaces old_string with new_string in the file.
	PatchOpReplace PatchOperation = "replace"
	// PatchOpDelete deletes the file.
	PatchOpDelete PatchOperation = "delete"
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
		return ToolOut{
			Content: fmt.Sprintf("Failed to parse input: %v", err),
			IsError: true,
		}
	}

	if args.Path == "" {
		return ToolOut{
			Content: "Path is required",
			IsError: true,
		}
	}

	// Resolve path
	path := args.Path
	if !filepath.IsAbs(path) && ctx.WorkingDir != "" {
		path = filepath.Join(ctx.WorkingDir, path)
	}

	switch args.Operation {
	case PatchOpCreate:
		return patchCreate(path, args.Content)
	case PatchOpReplace:
		return patchReplace(path, args.OldString, args.NewString)
	case PatchOpDelete:
		return patchDelete(path)
	default:
		return ToolOut{
			Content: fmt.Sprintf("Unknown operation: %s. Use 'create', 'replace', or 'delete'.", args.Operation),
			IsError: true,
		}
	}
}

func patchCreate(path, content string) ToolOut {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to create directory: %v", err),
			IsError: true,
		}
	}

	// Write file
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to write file: %v", err),
			IsError: true,
		}
	}

	lineCount := strings.Count(content, "\n") + 1
	return ToolOut{
		Content: fmt.Sprintf("Created %s (%d lines)", path, lineCount),
		IsError: false,
	}
}

func patchReplace(path, oldString, newString string) ToolOut {
	if oldString == "" {
		return ToolOut{
			Content: "old_string is required for replace operation",
			IsError: true,
		}
	}

	// Read existing file
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolOut{
				Content: fmt.Sprintf("File not found: %s. Use 'create' operation to create new files.", path),
				IsError: true,
			}
		}
		return ToolOut{
			Content: fmt.Sprintf("Failed to read file: %v", err),
			IsError: true,
		}
	}

	contentStr := string(content)

	// Check if old_string exists and is unique
	count := strings.Count(contentStr, oldString)
	if count == 0 {
		return ToolOut{
			Content: fmt.Sprintf("old_string not found in file. Make sure to include exact text including whitespace and indentation."),
			IsError: true,
		}
	}
	if count > 1 {
		return ToolOut{
			Content: fmt.Sprintf("old_string found %d times in file. It must be unique. Include more context to make it unique.", count),
			IsError: true,
		}
	}

	// Replace
	newContent := strings.Replace(contentStr, oldString, newString, 1)

	// Write back
	if err := os.WriteFile(path, []byte(newContent), 0o644); err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to write file: %v", err),
			IsError: true,
		}
	}

	oldLines := strings.Count(oldString, "\n") + 1
	newLines := strings.Count(newString, "\n") + 1
	return ToolOut{
		Content: fmt.Sprintf("Replaced %d lines with %d lines in %s", oldLines, newLines, path),
		IsError: false,
	}
}

func patchDelete(path string) ToolOut {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return ToolOut{
			Content: fmt.Sprintf("File not found: %s", path),
			IsError: true,
		}
	}

	// Delete file
	if err := os.Remove(path); err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to delete file: %v", err),
			IsError: true,
		}
	}

	return ToolOut{
		Content: fmt.Sprintf("Deleted %s", path),
		IsError: false,
	}
}
