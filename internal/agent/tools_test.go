package agent

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTools(t *testing.T) {
	t.Parallel()

	tools := CreateTools("")
	assert.Len(t, tools, 8)

	expectedTools := []string{"bash", "read", "patch", "think", "navigate", "read_schema", "ask_user", "web_search"}
	for _, name := range expectedTools {
		tool := GetToolByName(tools, name)
		require.NotNil(t, tool, "expected tool %s to exist", name)
		assert.Equal(t, "function", tool.Type)
	}
}

func TestGetToolByName(t *testing.T) {
	t.Parallel()

	tools := CreateTools("")

	tests := []struct {
		name         string
		tools        []*AgentTool
		toolName     string
		expectNil    bool
		expectedName string
	}{
		{
			name:         "finds existing tool",
			tools:        tools,
			toolName:     "bash",
			expectNil:    false,
			expectedName: "bash",
		},
		{
			name:      "returns nil for unknown tool",
			tools:     tools,
			toolName:  "unknown",
			expectNil: true,
		},
		{
			name:      "returns nil for empty name",
			tools:     tools,
			toolName:  "",
			expectNil: true,
		},
		{
			name:      "returns nil for empty tools slice",
			tools:     []*AgentTool{},
			toolName:  "bash",
			expectNil: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tool := GetToolByName(tc.tools, tc.toolName)
			if tc.expectNil {
				assert.Nil(t, tool)
			} else {
				require.NotNil(t, tool)
				assert.Equal(t, tc.expectedName, tool.Function.Name)
			}
		})
	}
}

func TestToolError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		format   string
		args     []any
		expected string
	}{
		{
			name:     "creates error with formatted message",
			format:   "Error: %s",
			args:     []any{"test"},
			expected: "Error: test",
		},
		{
			name:     "creates error without format arguments",
			format:   "Simple error",
			args:     nil,
			expected: "Simple error",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			result := toolError(tc.format, tc.args...)
			assert.True(t, result.IsError)
			assert.Equal(t, tc.expected, result.Content)
		})
	}
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
