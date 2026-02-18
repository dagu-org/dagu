package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "bash",
		Label:          "Bash",
		Description:    "Run shell commands",
		DefaultEnabled: true,
		Factory:        func(_ ToolConfig) *AgentTool { return NewBashTool() },
	})
}

const (
	defaultBashTimeout = 120 * time.Second
	maxBashTimeout     = 10 * time.Minute
	maxOutputLength    = 100000
)

// BashToolInput defines the input parameters for the bash tool.
type BashToolInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout,omitempty"`
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
		Audit: &AuditInfo{
			Action:          "bash_exec",
			DetailExtractor: ExtractFields("command"),
		},
	}
}

func bashRun(toolCtx ToolContext, input json.RawMessage) ToolOut {
	var args BashToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}
	if args.Command == "" {
		return toolError("Command is required")
	}
	if toolCtx.Role.IsSet() && !toolCtx.Role.CanExecute() {
		return toolError("Permission denied: bash requires execute permission")
	}

	return executeCommand(toolCtx, args)
}

func executeCommand(toolCtx ToolContext, args BashToolInput) ToolOut {
	timeout := resolveTimeout(args.Timeout)
	parentCtx := toolCtx.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(parentCtx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	if toolCtx.WorkingDir != "" {
		cmd.Dir = toolCtx.WorkingDir
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := buildOutput(stdout.String(), stderr.String())

	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return toolError("Command timed out after %v\n%s", timeout, output)
		}
		return toolError("Command failed: %v\n%s", err, output)
	}
	if output == "" {
		return ToolOut{Content: "(no output)"}
	}
	return ToolOut{Content: output}
}

func resolveTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultBashTimeout
	}
	return min(time.Duration(seconds)*time.Second, maxBashTimeout)
}

func buildOutput(stdout, stderr string) string {
	stdout = truncateOutput(stdout)
	stderr = truncateOutput(stderr)

	switch {
	case stderr == "":
		return stdout
	case stdout == "":
		return "STDERR:\n" + stderr
	default:
		return stdout + "\nSTDERR:\n" + stderr
	}
}

func truncateOutput(s string) string {
	if len(s) > maxOutputLength {
		return s[:maxOutputLength] + "\n... [output truncated]"
	}
	return s
}
