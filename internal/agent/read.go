package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	maxReadSize      = 1024 * 1024
	defaultLineLimit = 2000
)

// ReadToolInput defines the input parameters for the read tool.
type ReadToolInput struct {
	Path   string `json:"path"`
	Offset int    `json:"offset,omitempty"`
	Limit  int    `json:"limit,omitempty"`
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
		return toolError("Failed to parse input: %v", err)
	}

	if args.Path == "" {
		return toolError("Path is required")
	}

	path := resolvePath(args.Path, ctx.WorkingDir)

	if err := validateReadableFile(path, args.Path); err != nil {
		return err.(ToolOut)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return toolError("Failed to read file: %v", err)
	}

	return formatFileContent(string(content), args.Offset, args.Limit)
}

func validateReadableFile(path, displayPath string) any {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return toolError("File not found: %s", displayPath)
		}
		return toolError("Failed to access file: %v", err)
	}

	if info.IsDir() {
		return toolError("%s is a directory, not a file. Use bash with 'ls' to list directory contents.", displayPath)
	}

	if info.Size() > maxReadSize {
		return toolError("File too large (%d bytes). Maximum size is %d bytes. Use offset and limit to read portions.", info.Size(), maxReadSize)
	}

	return nil
}

func formatFileContent(content string, offsetArg, limitArg int) ToolOut {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)

	offset := max(0, offsetArg-1)
	limit := defaultLineLimit
	if limitArg > 0 {
		limit = limitArg
	}

	if offset >= totalLines {
		return toolError("Offset %d is beyond file length (%d lines)", offsetArg, totalLines)
	}

	end := min(offset+limit, totalLines)
	selectedLines := lines[offset:end]

	var result strings.Builder
	for i, line := range selectedLines {
		lineNum := offset + i + 1
		fmt.Fprintf(&result, "%6d\t%s\n", lineNum, line)
	}

	remainingLines := totalLines - end
	if remainingLines > 0 {
		fmt.Fprintf(&result, "\n... [%d more lines, use offset=%d to continue]", remainingLines, end+1)
	}

	return ToolOut{Content: result.String()}
}
