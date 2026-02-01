package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
	"github.com/google/uuid"
)

const (
	defaultBashTimeout = 120 * time.Second
	maxBashTimeout     = 10 * time.Minute
	maxOutputLength    = 100000
	approvalTimeout    = 5 * time.Minute
)

// dangerousPatterns contains command prefixes that require user approval.
var dangerousPatterns = []string{"rm ", "chmod ", "dagu start"}

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
	}
}

// commandRequiresApproval checks if a command matches dangerous patterns.
func commandRequiresApproval(cmd string) bool {
	cmd = strings.TrimSpace(cmd)
	for _, p := range dangerousPatterns {
		if strings.HasPrefix(cmd, p) ||
			strings.Contains(cmd, " "+p) ||
			strings.Contains(cmd, "|"+p) ||
			strings.Contains(cmd, "| "+p) ||
			strings.Contains(cmd, ";"+p) ||
			strings.Contains(cmd, "; "+p) {
			return true
		}
	}
	return false
}

// requestApproval asks the user to approve a command before execution.
func requestApproval(ctx ToolContext, cmd string) (bool, error) {
	if ctx.EmitUserPrompt == nil || ctx.WaitUserResponse == nil {
		return true, nil // No prompt mechanism, allow
	}

	promptID := uuid.New().String()
	ctx.EmitUserPrompt(UserPrompt{
		PromptID:   promptID,
		PromptType: PromptTypeCommandApproval,
		Question:   "Approve command?",
		Command:    cmd,
		WorkingDir: ctx.WorkingDir,
		Options: []UserPromptOption{
			{ID: "approve", Label: "Approve"},
			{ID: "reject", Label: "Reject"},
		},
	})

	// Timeout prevents indefinite blocking and channel leaks.
	parentCtx := ctx.Context
	if parentCtx == nil {
		parentCtx = context.Background()
	}
	timeoutCtx, cancel := context.WithTimeout(parentCtx, approvalTimeout)
	defer cancel()

	resp, err := ctx.WaitUserResponse(timeoutCtx, promptID)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return false, fmt.Errorf("approval timed out after %v", approvalTimeout)
		}
		return false, err
	}
	if resp.Cancelled {
		return false, nil
	}

	for _, id := range resp.SelectedOptionIDs {
		if id == "approve" {
			return true, nil
		}
	}
	return false, nil
}

func bashRun(toolCtx ToolContext, input json.RawMessage) ToolOut {
	var args BashToolInput
	if err := json.Unmarshal(input, &args); err != nil {
		return toolError("Failed to parse input: %v", err)
	}

	if args.Command == "" {
		return toolError("Command is required")
	}

	// Check if command requires approval (only when SafeMode is enabled)
	if toolCtx.SafeMode && commandRequiresApproval(args.Command) {
		approved, err := requestApproval(toolCtx, args.Command)
		if err != nil {
			return toolError("Approval failed: %v", err)
		}
		if !approved {
			return ToolOut{Content: "Command rejected by user"}
		}
	}

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
