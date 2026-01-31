package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTools(t *testing.T) {
	t.Parallel()

	tools := CreateTools()

	// Should return all expected tools
	assert.Len(t, tools, 6)

	expectedTools := []string{"bash", "read", "patch", "think", "navigate", "read_schema"}
	for _, name := range expectedTools {
		tool := GetToolByName(tools, name)
		require.NotNil(t, tool, "expected tool %s to exist", name)
		assert.Equal(t, "function", tool.Type)
	}
}

func TestGetToolByName(t *testing.T) {
	t.Parallel()

	tools := CreateTools()

	t.Run("finds existing tool", func(t *testing.T) {
		t.Parallel()

		tool := GetToolByName(tools, "bash")
		require.NotNil(t, tool)
		assert.Equal(t, "bash", tool.Function.Name)
	})

	t.Run("returns nil for unknown tool", func(t *testing.T) {
		t.Parallel()

		tool := GetToolByName(tools, "unknown")
		assert.Nil(t, tool)
	})

	t.Run("returns nil for empty name", func(t *testing.T) {
		t.Parallel()

		tool := GetToolByName(tools, "")
		assert.Nil(t, tool)
	})

	t.Run("returns nil for empty tools slice", func(t *testing.T) {
		t.Parallel()

		tool := GetToolByName([]*AgentTool{}, "bash")
		assert.Nil(t, tool)
	})
}

func TestToolError(t *testing.T) {
	t.Parallel()

	t.Run("creates error with message", func(t *testing.T) {
		t.Parallel()

		result := toolError("Error: %s", "test")
		assert.True(t, result.IsError)
		assert.Equal(t, "Error: test", result.Content)
	})

	t.Run("formats without arguments", func(t *testing.T) {
		t.Parallel()

		result := toolError("Simple error")
		assert.True(t, result.IsError)
		assert.Equal(t, "Simple error", result.Content)
	})
}

func TestResolvePath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		workingDir string
		expected   string
	}{
		{
			name:       "absolute path unchanged",
			path:       "/abs/path/file.txt",
			workingDir: "/work",
			expected:   "/abs/path/file.txt",
		},
		{
			name:       "relative path joined with workingDir",
			path:       "rel/path/file.txt",
			workingDir: "/work",
			expected:   "/work/rel/path/file.txt",
		},
		{
			name:       "relative path with empty workingDir unchanged",
			path:       "rel/path/file.txt",
			workingDir: "",
			expected:   "rel/path/file.txt",
		},
		{
			name:       "simple filename joined",
			path:       "file.txt",
			workingDir: "/home/user",
			expected:   "/home/user/file.txt",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := resolvePath(tc.path, tc.workingDir)
			assert.Equal(t, tc.expected, result)
		})
	}
}
