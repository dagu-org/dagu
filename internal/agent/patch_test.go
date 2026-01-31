package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPatchTool_Create(t *testing.T) {
	t.Parallel()

	t.Run("creates new file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "new.txt")

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "create", "content": "hello world"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Created")

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "hello world", string(content))
	})

	t.Run("creates parent directories", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "nested", "deep", "file.txt")

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "create", "content": "nested content"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "nested content", string(content))
	})

	t.Run("overwrites existing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("old content"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "create", "content": "new content"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "new content", string(content))
	})
}

func TestPatchTool_Replace(t *testing.T) {
	t.Parallel()

	t.Run("replaces unique string", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "replace", "old_string": "world", "new_string": "universe"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Replaced")

		content, err := os.ReadFile(filePath)
		require.NoError(t, err)
		assert.Equal(t, "hello universe", string(content))
	})

	t.Run("errors when old_string not found", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello world"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "replace", "old_string": "missing", "new_string": "replacement"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("errors when old_string found multiple times", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("hello hello hello"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "replace", "old_string": "hello", "new_string": "hi"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "3 times")
	})

	t.Run("errors when file not found", func(t *testing.T) {
		t.Parallel()

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "/nonexistent/file.txt", "operation": "replace", "old_string": "a", "new_string": "b"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("errors when old_string is empty", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "replace", "old_string": "", "new_string": "b"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})
}

func TestPatchTool_Delete(t *testing.T) {
	t.Parallel()

	t.Run("deletes existing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "delete-me.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "operation": "delete"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "Deleted")

		_, err := os.Stat(filePath)
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("errors when file not found", func(t *testing.T) {
		t.Parallel()

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "/nonexistent/file.txt", "operation": "delete"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})
}

func TestPatchTool_Validation(t *testing.T) {
	t.Parallel()

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "", "operation": "create", "content": "test"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("unknown operation returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "/test.txt", "operation": "unknown"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "Unknown operation")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewPatchTool()
		input := json.RawMessage(`{invalid}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})
}

func TestPatchTool_WorkingDirectory(t *testing.T) {
	t.Parallel()

	t.Run("uses working directory for relative paths", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		tool := NewPatchTool()
		input := json.RawMessage(`{"path": "test.txt", "operation": "create", "content": "content"}`)
		ctx := ToolContext{WorkingDir: dir}

		result := tool.Run(ctx, input)

		assert.False(t, result.IsError)

		content, err := os.ReadFile(filepath.Join(dir, "test.txt"))
		require.NoError(t, err)
		assert.Equal(t, "content", string(content))
	})
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
		result := countLines(tc.input)
		assert.Equal(t, tc.expected, result, "for input %q", tc.input)
	}
}
