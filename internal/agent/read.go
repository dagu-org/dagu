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
	maxReadSize  = 1024 * 1024 // 1MB max file size
	defaultLines = 2000       // Default max lines to read
)

// ReadToolInput is the input schema for the read tool.
type ReadToolInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"` // Line offset (1-based)
	Limit  int    `json:"limit,omitempty"`  // Max lines to read
}

// NewReadTool creates a new read tool for file reading.
func NewReadTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "read",
				Description: "Read the contents of a file. Returns the file content with line numbers.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"path": map[string]any{
							"type":        "string",
							"description": "The path to the file to read (absolute or relative to working directory)",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Optional line offset to start reading from (1-based)",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Optional maximum number of lines to read (default: 2000)",
						},
					},
					"required": []string{"path"},
				},
			},
		},
		Run: readRun,
	}
}

func readRun(ctx ToolContext, input json.RawMessage) ToolOut {
	var args ReadToolInput
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

	// Check if file exists
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ToolOut{
				Content: fmt.Sprintf("File not found: %s", args.Path),
				IsError: true,
			}
		}
		return ToolOut{
			Content: fmt.Sprintf("Failed to access file: %v", err),
			IsError: true,
		}
	}

	if info.IsDir() {
		return ToolOut{
			Content: fmt.Sprintf("%s is a directory, not a file. Use bash with 'ls' to list directory contents.", args.Path),
			IsError: true,
		}
	}

	// Check file size
	if info.Size() > maxReadSize {
		return ToolOut{
			Content: fmt.Sprintf("File too large (%d bytes). Maximum size is %d bytes. Use offset and limit to read portions.", info.Size(), maxReadSize),
			IsError: true,
		}
	}

	// Read file
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to read file: %v", err),
			IsError: true,
		}
	}

	// Split into lines
	lines := strings.Split(string(content), "\n")

	// Apply offset and limit
	offset := 0
	if args.Offset > 0 {
		offset = args.Offset - 1 // Convert to 0-based
	}
	limit := defaultLines
	if args.Limit > 0 {
		limit = args.Limit
	}

	// Validate offset
	if offset >= len(lines) {
		return ToolOut{
			Content: fmt.Sprintf("Offset %d is beyond file length (%d lines)", args.Offset, len(lines)),
			IsError: true,
		}
	}

	// Apply offset and limit
	end := offset + limit
	if end > len(lines) {
		end = len(lines)
	}
	selectedLines := lines[offset:end]

	// Format with line numbers
	var result strings.Builder
	for i, line := range selectedLines {
		lineNum := offset + i + 1 // 1-based line numbers
		result.WriteString(fmt.Sprintf("%6d\t%s\n", lineNum, line))
	}

	// Add truncation notice if needed
	if end < len(lines) {
		result.WriteString(fmt.Sprintf("\n... [%d more lines, use offset=%d to continue]", len(lines)-end, end+1))
	}

	return ToolOut{
		Content: result.String(),
		IsError: false,
	}
}
