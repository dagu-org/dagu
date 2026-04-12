// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func backgroundCtx() ToolContext {
	return ToolContext{Context: context.Background()}
}

func createPathHelper(tb testing.TB, dir string) string {
	tb.Helper()

	name := "path-helper"
	path := filepath.Join(dir, name)
	content := "#!/bin/sh\nprintf 'path-helper-marker\\n'\n"
	if runtime.GOOS == "windows" {
		path += ".cmd"
		content = "@echo off\r\necho path-helper-marker\r\n"
	}

	require.NoError(tb, os.WriteFile(path, []byte(content), 0o700))
	return name
}

func noBashPath() (string, bool) {
	return "", false
}

func bashPathForTests(tb testing.TB) bashPathFinder {
	tb.Helper()

	path, ok := findBashPath()
	if !ok {
		tb.Skip("bash is not available")
	}
	return func() (string, bool) { return path, true }
}

func runCommandWithFinder(
	tb testing.TB,
	ctx context.Context,
	workDir, command string,
	timeout int,
	findPath bashPathFinder,
) ToolOut {
	tb.Helper()

	return executeCommandWithFinder(ToolContext{
		Context:    ctx,
		WorkingDir: workDir,
	}, BashToolInput{
		Command: command,
		Timeout: timeout,
	}, findPath)
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

	t.Run("rejects role without execute permission", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "echo test"}`)

		result := tool.Run(ToolContext{
			Context: context.Background(),
			Role:    auth.RoleViewer,
		}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "requires execute permission")
	})
}

func TestBashTool_Timeout(t *testing.T) {
	t.Parallel()

	t.Run("respects custom timeout", func(t *testing.T) {
		t.Parallel()

		tool := NewBashTool()
		input := json.RawMessage(`{"command": "while :; do :; done", "timeout": 1}`)

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
		input := json.RawMessage(`{"command": "while :; do :; done", "timeout": 10}`)

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		result := tool.Run(ToolContext{Context: ctx}, input)
		elapsed := time.Since(start)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "canceled")
		assert.Less(t, elapsed, time.Second)
	})
}

func TestExecuteCommand_InterpreterFallback(t *testing.T) {
	t.Parallel()

	t.Run("executes simple echo", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `echo hello`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello")
	})

	t.Run("returns no output for silent commands", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `true`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Equal(t, "(no output)", result.Content)
	})

	t.Run("captures stderr", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `echo error >&2`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "STDERR")
		assert.Contains(t, result.Content, "error")
	})

	t.Run("reports exit failure", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `exit 1`, 0, noBashPath)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "failed")
	})

	t.Run("supports pipelines with builtins", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `printf 'hello\n' | while IFS= read -r line; do echo "${line}!"; done`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello!")
	})

	t.Run("respects working directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("ok"), 0o600))

		result := runCommandWithFinder(t, context.Background(), dir, `test -f marker.txt && echo found`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "found")
	})

	t.Run("supports variable expansion", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `X=hello; echo "$X"`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello")
	})

	t.Run("supports command substitution", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `echo "$(echo nested)"`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "nested")
	})

	t.Run("supports bash syntax mode", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `value=hello; [[ "$value" == hello ]] && echo yes`, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "yes")
	})

	t.Run("runs PATH-installed executables", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		helper := createPathHelper(t, dir)
		command := fmt.Sprintf("PATH=%q; %s", dir+string(os.PathListSeparator)+"$PATH", helper)

		result := runCommandWithFinder(t, context.Background(), "", command, 0, noBashPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "path-helper-marker")
	})

	t.Run("reports parse errors", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `if then`, 0, noBashPath)

		assert.True(t, result.IsError)
		assert.Contains(t, strings.ToLower(result.Content), "parse")
	})
}

func TestExecuteCommand_InterpreterFallback_Timeout(t *testing.T) {
	t.Parallel()

	t.Run("internal timeout is reported as timeout", func(t *testing.T) {
		t.Parallel()

		start := time.Now()
		result := runCommandWithFinder(t, context.Background(), "", `while :; do :; done`, 1, noBashPath)
		elapsed := time.Since(start)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "timed out")
		assert.Less(t, elapsed, 2*time.Second)
	})

	t.Run("parent deadline is reported as cancellation", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		start := time.Now()
		result := runCommandWithFinder(t, ctx, "", `while :; do :; done`, 10, noBashPath)
		elapsed := time.Since(start)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "canceled")
		assert.Contains(t, result.Content, "deadline exceeded")
		assert.Less(t, elapsed, time.Second)
	})
}

func TestExecuteCommand_BashPath(t *testing.T) {
	t.Parallel()

	findPath := bashPathForTests(t)

	t.Run("executes simple echo", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `echo hello`, 0, findPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello")
	})

	t.Run("captures stderr", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `echo error >&2`, 0, findPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "STDERR")
		assert.Contains(t, result.Content, "error")
	})

	t.Run("reports exit failure", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `exit 1`, 0, findPath)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "failed")
	})

	t.Run("supports pipelines", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `printf 'hello\n' | while IFS= read -r line; do echo "${line}!"; done`, 0, findPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "hello!")
	})

	t.Run("respects working directory", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "marker.txt"), []byte("ok"), 0o600))

		result := runCommandWithFinder(t, context.Background(), dir, `test -f marker.txt && echo found`, 0, findPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "found")
	})

	t.Run("supports bash syntax", func(t *testing.T) {
		t.Parallel()

		result := runCommandWithFinder(t, context.Background(), "", `value=hello; [[ "$value" == hello ]] && echo yes`, 0, findPath)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "yes")
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

func TestCappedWriter(t *testing.T) {
	t.Parallel()

	t.Run("preserves short output", func(t *testing.T) {
		t.Parallel()

		writer := newCappedWriter(16)
		n, err := writer.Write([]byte("short"))

		require.NoError(t, err)
		assert.Equal(t, 5, n)
		assert.Equal(t, "short", writer.String())
	})

	t.Run("caps long output and appends marker", func(t *testing.T) {
		t.Parallel()

		writer := newCappedWriter(len(outputTruncationMarker) + 5)
		n, err := writer.Write([]byte(strings.Repeat("x", len(outputTruncationMarker)+10)))

		require.NoError(t, err)
		assert.Equal(t, len(outputTruncationMarker)+10, n)
		assert.Equal(t, strings.Repeat("x", 5)+outputTruncationMarker, writer.String())
	})
}
