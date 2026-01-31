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

func createTestFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o600))
	return filePath
}

func readInput(path string) json.RawMessage {
	input := map[string]string{"path": path}
	b, _ := json.Marshal(input)
	return b
}

func TestReadTool_Run(t *testing.T) {
	t.Parallel()
	tool := NewReadTool()

	t.Run("reads existing file", func(t *testing.T) {
		t.Parallel()
		filePath := createTestFile(t, "line1\nline2\nline3")

		result := tool.Run(ToolContext{}, readInput(filePath))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "line1")
		assert.Contains(t, result.Content, "line2")
		assert.Contains(t, result.Content, "line3")
	})

	t.Run("includes line numbers", func(t *testing.T) {
		t.Parallel()
		filePath := createTestFile(t, "first\nsecond")

		result := tool.Run(ToolContext{}, readInput(filePath))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "1\t")
		assert.Contains(t, result.Content, "2\t")
	})

	t.Run("file not found returns error", func(t *testing.T) {
		t.Parallel()

		result := tool.Run(ToolContext{}, readInput("/nonexistent/path/file.txt"))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "not found")
	})

	t.Run("directory returns error", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		result := tool.Run(ToolContext{}, readInput(dir))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "directory")
	})

	t.Run("empty path returns error", func(t *testing.T) {
		t.Parallel()

		result := tool.Run(ToolContext{}, json.RawMessage(`{"path": ""}`))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "required")
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		t.Parallel()

		result := tool.Run(ToolContext{}, json.RawMessage(`{invalid}`))

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "parse")
	})

	t.Run("respects offset parameter", func(t *testing.T) {
		t.Parallel()
		filePath := createTestFile(t, "line1\nline2\nline3\nline4\nline5")
		input := json.RawMessage(`{"path": "` + filePath + `", "offset": 3}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "3\t")
		assert.Contains(t, result.Content, "line3")
		assert.NotContains(t, result.Content, "1\tline1")
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		t.Parallel()
		filePath := createTestFile(t, "line1\nline2\nline3\nline4\nline5")
		input := json.RawMessage(`{"path": "` + filePath + `", "limit": 2}`)

		result := tool.Run(ToolContext{}, input)

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "more lines")
	})

	t.Run("offset beyond file length returns error", func(t *testing.T) {
		t.Parallel()
		filePath := createTestFile(t, "short")
		input := json.RawMessage(`{"path": "` + filePath + `", "offset": 100}`)

		result := tool.Run(ToolContext{}, input)

		assert.True(t, result.IsError)
		assert.Contains(t, result.Content, "beyond")
	})

	t.Run("uses working directory for relative paths", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("content"), 0o600))
		ctx := ToolContext{WorkingDir: dir}

		result := tool.Run(ctx, json.RawMessage(`{"path": "test.txt"}`))

		assert.False(t, result.IsError)
		assert.Contains(t, result.Content, "content")
	})
}

func TestReadTool_LargeFile(t *testing.T) {
	t.Parallel()

	tool := NewReadTool()
	largeContent := strings.Repeat("x", 1024*1024+100)
	filePath := createTestFile(t, largeContent)

	result := tool.Run(ToolContext{}, readInput(filePath))

	assert.True(t, result.IsError)
	assert.Contains(t, result.Content, "too large")
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

	tests := []struct {
		name         string
		content      string
		offset       int
		limit        int
		wantContains []string
		wantExcludes []string
	}{
		{
			name:         "formats with line numbers",
			content:      "a\nb\nc",
			wantContains: []string{"1\t", "2\t", "3\t"},
		},
		{
			name:         "applies offset correctly",
			content:      "a\nb\nc\nd",
			offset:       2,
			wantContains: []string{"2\tb"},
			wantExcludes: []string{"1\ta"},
		},
		{
			name:         "applies limit correctly",
			content:      "a\nb\nc\nd\ne",
			limit:        2,
			wantContains: []string{"more lines"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := formatFileContent(tc.content, tc.offset, tc.limit)

			assert.False(t, result.IsError)
			for _, want := range tc.wantContains {
				assert.Contains(t, result.Content, want)
			}
			for _, exclude := range tc.wantExcludes {
				assert.NotContains(t, result.Content, exclude)
			}
		})
	}
}
