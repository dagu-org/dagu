package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func patchInput(path, operation string, extra ...string) json.RawMessage {
	base := fmt.Sprintf(`{"path": %q, "operation": %q`, path, operation)
	for i := 0; i < len(extra)-1; i += 2 {
		base += fmt.Sprintf(`, %q: %q`, extra[i], extra[i+1])
	}
	return json.RawMessage(base + "}")
}

func TestPatchTool_Create(t *testing.T) {
	t.Parallel()
	tool := NewPatchTool()

	t.Run("creates new file", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "new.txt")
		result := tool.Run(ToolContext{}, patchInput(filePath, "create", "content", "hello world"))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Created")

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("creates parent directories", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "nested", "deep", "file.txt")
		result := tool.Run(ToolContext{}, patchInput(filePath, "create", "content", "nested content"))

		assert.False(t, result.IsError)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "nested content", string(content))
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "existing.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("old content"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "create", "content", "new content"))

		assert.False(t, result.IsError)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "new content", string(content))
	})
}

func TestPatchTool_Replace(t *testing.T) {
	t.Parallel()
	tool := NewPatchTool()

	t.Run("replaces unique string", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "replace", "old_string", "world", "new_string", "universe"))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Replaced")

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "hello universe", string(content))
	})

	t.Run("errors when old_string not found", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "replace", "old_string", "missing", "new_string", "replacement"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("errors when old_string found multiple times", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello hello hello"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "replace", "old_string", "hello", "new_string", "hi"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "3 times")
	})

	t.Run("errors when file not found", func(t *testing.T) {
		t.Parallel()

		result := tool.Run(ToolContext{}, patchInput("/nonexistent/file.txt", "replace", "old_string", "a", "new_string", "b"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("errors when old_string is empty", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "replace", "old_string", "", "new_string", "b"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})
}

func TestPatchTool_Delete(t *testing.T) {
	t.Parallel()
	tool := NewPatchTool()

	t.Run("deletes existing file", func(t *testing.T) {
		t.Parallel()

		filePath := filepath.Join(t.TempDir(), "delete-me.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

		result := tool.Run(ToolContext{}, patchInput(filePath, "delete"))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Deleted")

		_, err := os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("errors when file not found", func(t *testing.T) {
		t.Parallel()

		result := tool.Run(ToolContext{}, patchInput("/nonexistent/file.txt", "delete"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})
}

func TestPatchTool_Validation(t *testing.T) {
	t.Parallel()
	tool := NewPatchTool()

	tests := []struct {
		name     string
		input    json.RawMessage
		contains string
	}{
		{
			name:     "empty path returns error",
			input:    patchInput("", "create", "content", "test"),
			contains: "required",
		},
		{
			name:     "unknown operation returns error",
			input:    patchInput("/test.txt", "unknown"),
			contains: "Unknown operation",
		},
		{
			name:     "invalid JSON returns error",
			input:    json.RawMessage(`{invalid}`),
			contains: "parse",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := tool.Run(ToolContext{}, tc.input)
			assert.True(t, result.IsError)
			assert.Contains(t, result.Content, tc.contains)
		})
	}
}

func TestPatchTool_WorkingDirectory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	result := NewPatchTool().Run(
		ToolContext{WorkingDir: dir},
		patchInput("test.txt", "create", "content", "content"),
	)

	assert.False(t, result.IsError)

	content, err := os.ReadFile(filepath.Join(dir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "content", string(content))
}

func TestNewPatchTool(t *testing.T) {
	t.Parallel()

	tool := NewPatchTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "patch", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Run)
}

func TestCountLines(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected int
	}{
		{"", 1},
		{"single line", 1},
		{"line1\nline2", 2},
		{"line1\nline2\nline3", 3},
		{"\n\n", 3},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, countLines(tc.input))
		})
	}
}
