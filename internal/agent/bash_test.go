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
