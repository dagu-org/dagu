package executor

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
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

func TestValidateSSHStep(t *testing.T) {
	t.Parallel()

	// Valid step
	step := core.Step{
		Name:    "valid-ssh-step",
		Command: "echo 'hello'",
	}
	err := validateSSHStep(step)
	require.NoError(t, err)

	// Verify that script field is not allowed
	step.Script = "echo 'hello'"
	err = validateSSHStep(step)
	require.Error(t, err)
	require.Contains(t, err.Error(), "script field is not supported with SSH executor")
}
