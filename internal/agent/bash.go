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
	maxBashTimeout     = 10 * time.Minute
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
		return bashError("Failed to parse input: %v", err)
	}

	if args.Command == "" {
		return bashError("Command is required")
	}

	timeout := calcTimeout(args.Timeout)
	cmdCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "bash", "-c", args.Command)
	if ctx.WorkingDir != "" {
		cmd.Dir = ctx.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := buildOutput(stdout.String(), stderr.String())

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return bashError("Command timed out after %v\n%s", timeout, output)
		}
		return bashError("Command failed: %v\n%s", err, output)
	}

	if output == "" {
		output = "(no output)"
	}

	return ToolOut{Content: output}
}

func calcTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultBashTimeout
	}
	return min(time.Duration(seconds)*time.Second, maxBashTimeout)
}

func buildOutput(stdoutStr, stderrStr string) string {
	stdoutStr = truncateOutput(stdoutStr)
	stderrStr = truncateOutput(stderrStr)

	var output strings.Builder
	output.WriteString(stdoutStr)

	if stderrStr != "" {
		if output.Len() > 0 {
			output.WriteString("\n")
		}
		output.WriteString("STDERR:\n")
		output.WriteString(stderrStr)
	}

	return output.String()
}

func truncateOutput(s string) string {
	if len(s) > maxOutputLength {
		return s[:maxOutputLength] + "\n... [output truncated]"
	}
	return s
}

func bashError(format string, args ...any) ToolOut {
	return ToolOut{
		Content: fmt.Sprintf(format, args...),
		IsError: true,
	}
}
