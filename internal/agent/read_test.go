package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadTool_Run(t *testing.T) {
	t.Parallel()

	t.Run("reads existing file", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		content := "line1\nline2\nline3"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "line1")
		assert.Contains(t, result.Content, "line2")
		assert.Contains(t, result.Content, "line3")
	})

	t.Run("includes line numbers", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("first\nsecond"), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `"}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "1\t")
		assert.Contains(t, result.Content, "2\t")
	})

	t.Run("file not found returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "/nonexistent/path/file.txt"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("directory returns error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + dir + `"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "directory")
	})

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewReadTool()
		input := json.RawMessage(`{"path": ""}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		tool := NewReadTool()
		input := json.RawMessage(`{invalid}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})

	t.Run("respects offset parameter", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		content := "line1\nline2\nline3\nline4\nline5"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "offset": 3}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		// Should start from line 3
		assert.Contains(t, result.Content, "3\t")
		assert.Contains(t, result.Content, "line3")
		// Should not contain lines 1-2
		assert.NotContains(t, result.Content, "1\tline1")
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		content := "line1\nline2\nline3\nline4\nline5"
		require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "limit": 2}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		// Should show "more lines" indicator
		assert.Contains(t, result.Content, "more lines")
	})

	t.Run("offset beyond file length returns error", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("short"), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `", "offset": 100}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "beyond")
	})

	t.Run("uses working directory for relative paths", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "test.txt"}`)
		ctx := ToolContext{WorkingDir: dir}

		result := tool.Run(ctx, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "content")
	})
}

func TestReadTool_LargeFile(t *testing.T) {
	t.Parallel()

	t.Run("rejects file larger than max size", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		filePath := filepath.Join(dir, "large.txt")

		// Create a file larger than maxReadSize (1MB)
		largeContent := strings.Repeat("x", 1024*1024+100)
		require.NoError(t, os.WriteFile(filePath, []byte(largeContent), 0o600))

		tool := NewReadTool()
		input := json.RawMessage(`{"path": "` + filePath + `"}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "too large")
	})
}

func TestNewReadTool(t *testing.T) {
	t.Parallel()

	tool := NewReadTool()

	assert.Equal(t, "function", tool.Type)
	assert.Equal(t, "read", tool.Function.Name)
	assert.NotEmpty(t, tool.Function.Description)
	assert.NotNil(t, tool.Run)
}

func TestFormatFileContent(t *testing.T) {
	t.Parallel()

	t.Run("formats with line numbers", func(t *testing.T) {
		t.Parallel()

		result := formatFileContent("a\nb\nc", 0, 0)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "1\t")
		assert.Contains(t, result.Content, "2\t")
		assert.Contains(t, result.Content, "3\t")
	})

	t.Run("applies offset correctly", func(t *testing.T) {
		t.Parallel()

		result := formatFileContent("a\nb\nc\nd", 2, 0)

		assert.False(t, result.IsError)
		// Line numbers should start from 2
		assert.Contains(t, result.Content, "2\tb")
		assert.NotContains(t, result.Content, "1\ta")
	})

	t.Run("applies limit correctly", func(t *testing.T) {
		t.Parallel()

		result := formatFileContent("a\nb\nc\nd\ne", 0, 2)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "more lines")
	})
}
