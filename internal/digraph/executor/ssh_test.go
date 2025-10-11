package executor

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/require"
)

func TestSSHExecutor(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		step := digraph.Step{
			Name: "ssh-exec",
			ExecutorConfig: digraph.ExecutorConfig{
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
		_, err := newSSHExec(ctx, step)
		require.NoError(t, err)
	})
}

func TestValidateSSHStep(t *testing.T) {
	t.Parallel()

	// Valid step
	step := digraph.Step{
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
