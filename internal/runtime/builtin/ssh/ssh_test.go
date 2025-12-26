package ssh

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSHExecutor(t *testing.T) {
	t.Parallel()

	step := core.Step{
		Name: "ssh-exec",
		ExecutorConfig: core.ExecutorConfig{
			Type: "ssh",
			Config: map[string]any{
				"User":     "testuser",
				"IP":       "testip",
				"Port":     25,
				"Password": "testpassword",
			},
		},
	}
	ctx := context.Background()
	_, err := NewSSHExecutor(ctx, step)
	require.NoError(t, err)
}

func TestSSHCommandEscaping(t *testing.T) {
	tests := []struct {
		name     string
		command  string
		args     []string
		expected string
	}{
		{
			name:     "Simple command",
			command:  "ls",
			args:     nil,
			expected: "ls",
		},
		{
			name:     "Command with space",
			command:  "echo",
			args:     []string{"hello world"},
			expected: "echo 'hello world'",
		},
		{
			name:     "Command with special characters",
			command:  "echo",
			args:     []string{"$HOME", "quote'quote"},
			expected: "echo '$HOME' 'quote'\\''quote'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := cmdutil.ShellQuote(tt.command)
			if len(tt.args) > 0 {
				actual += " " + cmdutil.ShellQuoteArgs(tt.args)
			}
			assert.Equal(t, tt.expected, actual)
		})
	}
}
