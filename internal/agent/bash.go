package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	defaultBashTimeout = 120 * time.Second
	maxOutputLength    = 100000 // 100KB max output
)

// BashToolInput is the input schema for the bash tool.
type BashToolInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"` // timeout in seconds
}

// NewBashTool creates a new bash tool for shell command execution.
func NewBashTool() *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "bash",
				Description: "Execute a shell command and return the output. Use this for file listing, git operations, running scripts, etc.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The shell command to execute",
						},
						"timeout": map[string]any{
							"type":        "integer",
							"description": "Optional timeout in seconds (default: 120, max: 600)",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		Run: bashRun,
	}
}

func bashRun(ctx ToolContext, input json.RawMessage) ToolOut {
	var args BashToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolOut{
			Content: fmt.Sprintf("Failed to parse input: %v", err),
			IsError: true,
		}
	}

	if args.Command == "" {
		return ToolOut{
			Content: "Command is required",
			IsError: true,
		}
	}

	// Determine timeout
	timeout := defaultBashTimeout
	if args.Timeout > 0 {
		timeout = time.Duration(args.Timeout) * time.Second
		// Cap at 10 minutes
		if timeout > 10*time.Minute {
			timeout = 10 * time.Minute
		}
	}

	// Create context with timeout
	cmdCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Execute the command
	cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
	if ctx.WorkingDir != "" {
		cmd.Dir = ctx.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build output
	var output strings.Builder

	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Truncate if necessary
	if len(stdoutStr) > maxOutputLength {
		stdoutStr = stdoutStr[:maxOutputLength] + "\n... [output truncated]"
	}
	if len(stderrStr) > maxOutputLength {
		stderrStr = stderrStr[:maxOutputLength] + "\n... [output truncated]"
	}

	if stdoutStr != "" {
		output.WriteString(stdoutStr)
	}
	if stderrStr != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(stderrStr)
	}

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return ToolOut{
				Content: fmt.Sprintf("Command timed out after %v\n%s", timeout, output.String()),
				IsError: true,
			}
		}
		return ToolOut{
			Content: fmt.Sprintf("Command failed: %v\n%s", err, output.String()),
			IsError: true,
		}
	}

	result := output.String()
	if result == "" {
		result = "(no output)"
	}

	return ToolOut{
		Content: result,
		IsError: false,
	}
}
