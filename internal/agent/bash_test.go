package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func backgroundCtx() ToolContext {
	return ToolContext{Context: context.Background()}
}

func TestBashTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("executes simple command", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "echo hello"}`)

		result := tool.Run(backgroundCtx(), input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello")
	})

	t.Run("empty command returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": ""}`)

		result := tool.Run(backgroundCtx(), input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("returns no output message for silent commands", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "true"}`)

		result := tool.Run(backgroundCtx(), input)

		assert.False(t, result.IsError)
		assert.Equal(t, "(no output)", result.Content)
	})

	t.Run("captures stderr", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "echo error >&2"}`)

		result := tool.Run(backgroundCtx(), input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "STDERR")
		assert.Contains(t, result.Content, "error")
	})

	t.Run("reports command failure", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "exit 1"}`)

		result := tool.Run(backgroundCtx(), input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "failed")
	})

	t.Run("respects working directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "pwd"}`)
		ctx := ToolContext{
			Context:    context.Background(),
			WorkingDir: dir,
		}

		result := tool.Run(ctx, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, dir)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{invalid}`)

		result := tool.Run(backgroundCtx(), input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})

	t.Run("works with nil context", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "echo test"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "test")
	})
}

func TestBashTool_Timeout(t *testing.T) {
	t.Parallel()

	t.Run("respects custom timeout", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "sleep 2", "timeout": 1}`)

		start := time.Now()
		result := tool.Run(backgroundCtx(), input)
		elapsed := time.Since(start)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "timed out")
		assert.Less(t, elapsed, 2*time.Second)
	})

	t.Run("context cancellation stops command", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "sleep 10"}`)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		result := tool.Run(ToolContext{Context: ctx}, input)
		elapsed := time.Since(start)

		assert.True(t, result.IsError)
		assert.Less(t, elapsed, time.Second)
	})
}

func TestResolveTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		seconds  int
		expected time.Duration
	}{
		{
			name:     "zero uses default",
			seconds:  0,
			expected: defaultBashTimeout,
		},
		{
			name:     "negative uses default",
			seconds:  -1,
			expected: defaultBashTimeout,
		},
		{
			name:     "valid timeout",
			seconds:  60,
			expected: 60 * time.Second,
		},
		{
			name:     "large timeout capped at max",
			seconds:  700,
			expected: maxBashTimeout,
		},
		{
			name:     "exactly at max",
			seconds:  600,
			expected: 600 * time.Second,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := resolveTimeout(tc.seconds)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBuildOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		stdout   string
		stderr   string
		expected string
	}{
		{
			name:     "stdout only",
			stdout:   "output",
			stderr:   "",
			expected: "output",
		},
		{
			name:     "stderr only",
			stdout:   "",
			stderr:   "error",
			expected: "STDERR:\nerror",
		},
		{
			name:     "both stdout and stderr",
			stdout:   "output",
			stderr:   "error",
			expected: "output\nSTDERR:\nerror",
		},
		{
			name:     "both empty",
			stdout:   "",
			stderr:   "",
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := buildOutput(tc.stdout, tc.stderr)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTruncateOutput(t *testing.T) {
	t.Parallel()

	t.Run("short string unchanged", func(t *testing.T) {
		t.Parallel()

		result := truncateOutput("short")
		assert.Equal(t, "short", result)
	})

	t.Run("long string truncated", func(t *testing.T) {
		t.Parallel()

		longString := strings.Repeat("x", maxOutputLength+100)
		result := truncateOutput(longString)

		assert.Len(t, result, maxOutputLength+len("\n... [output truncated]"))
		assert.Contains(t, result, "truncated")
	})

	t.Run("exactly at limit unchanged", func(t *testing.T) {
		t.Parallel()

		exactString := strings.Repeat("x", maxOutputLength)
		result := truncateOutput(exactString)

		assert.Equal(t, exactString, result)
	})
}

func TestCommandRequiresApproval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		cmd      string
		expected bool
	}{
		// Safe commands
		{"ls command", "ls -la", false},
		{"echo command", "echo hello", false},
		{"cat command", "cat file.txt", false},
		{"pwd command", "pwd", false},
		{"git status", "git status", false},

		// rm commands (dangerous)
		{"rm direct", "rm file.txt", true},
		{"rm with flags", "rm -rf /tmp/test", true},
		{"rm in pipe", "find . | rm somefile", true},
		{"rm in pipe with space", "find . | rm somefile", true},
		{"rm after semicolon", "echo test;rm file", true},
		{"rm after semicolon with space", "echo test; rm file", true},
		{"rm with && operator", "echo test && rm file", true},
		{"command then rm", "ls && rm file", true},

		// chmod commands (dangerous)
		{"chmod direct", "chmod 755 script.sh", true},
		{"chmod in pipe", "ls |chmod 644 file", true},
		{"chmod after semicolon", "echo test;chmod 777 file", true},

		// dagu start commands (dangerous)
		{"dagu start direct", "dagu start mydag", true},
		{"dagu start with path", "dagu start /path/to/dag", true},

		// Edge cases
		{"empty command", "", false},
		{"whitespace only", "   ", false},
		{"similar but safe", "remove file.txt", false},
		{"chmod in string", "echo 'chmod 755'", false}, // This is safe - just echo
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := commandRequiresApproval(tc.cmd)
			assert.Equal(t, tc.expected, result, "command: %q", tc.cmd)
		})
	}
}

func TestBashTool_SafeMode_SkipsApproval(t *testing.T) {
	t.Parallel()

	tool := NewBashTool()
	// Dangerous command with SafeMode=false (default)
	input := json.RawMessage(`{"command": "rm -rf /tmp/nonexistent_test_file_12345"}`)

	// Should execute without calling approval (promptCalled stays false)
	promptCalled := false
	ctx := ToolContext{
		Context:        context.Background(),
		SafeMode:       false,
		EmitUserPrompt: func(_ UserPrompt) { promptCalled = true },
	}

	_ = tool.Run(ctx, input)

	assert.False(t, promptCalled, "approval should not be requested when SafeMode is false")
}

func TestBashTool_SafeMode_RequestsApproval(t *testing.T) {
	t.Parallel()

	tool := NewBashTool()
	input := json.RawMessage(`{"command": "rm testfile"}`)

	var capturedPrompt UserPrompt
	responseCh := make(chan UserPromptResponse, 1)
	responseCh <- UserPromptResponse{
		PromptID:          "test-id",
		SelectedOptionIDs: []string{"approve"},
	}

	ctx := ToolContext{
		Context:  context.Background(),
		SafeMode: true,
		EmitUserPrompt: func(p UserPrompt) {
			capturedPrompt = p
		},
		WaitUserResponse: func(_ context.Context, _ string) (UserPromptResponse, error) {
			return <-responseCh, nil
		},
	}

	_ = tool.Run(ctx, input)

	assert.Equal(t, PromptTypeCommandApproval, capturedPrompt.PromptType)
	assert.Equal(t, "rm testfile", capturedPrompt.Command)
	assert.Equal(t, "Approve command?", capturedPrompt.Question)
}

func TestBashTool_SafeMode_UserRejects(t *testing.T) {
	t.Parallel()

	tool := NewBashTool()
	input := json.RawMessage(`{"command": "rm testfile"}`)

	ctx := ToolContext{
		Context:        context.Background(),
		SafeMode:       true,
		EmitUserPrompt: func(_ UserPrompt) {},
		WaitUserResponse: func(_ context.Context, _ string) (UserPromptResponse, error) {
			return UserPromptResponse{SelectedOptionIDs: []string{"reject"}}, nil
		},
	}

	result := tool.Run(ctx, input)

	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "rejected by user")
}

func TestBashTool_SafeMode_SafeCommandNoApproval(t *testing.T) {
	t.Parallel()

	tool := NewBashTool()
	input := json.RawMessage(`{"command": "echo hello"}`)

	promptCalled := false
	ctx := ToolContext{
		Context:        context.Background(),
		SafeMode:       true,
		EmitUserPrompt: func(_ UserPrompt) { promptCalled = true },
	}

	result := tool.Run(ctx, input)

	assert.False(t, promptCalled, "safe commands should not request approval even with SafeMode=true")
	assert.False(t, result.IsError)
	assert.Contains(t, result.Content, "hello")
}
