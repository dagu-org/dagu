package executor

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/stretchr/testify/assert"
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

	t.Run("ValidateStepWithScript", func(t *testing.T) {
		step := digraph.Step{
			Name:   "ssh-with-script",
			Script: "echo 'test'",
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "ssh",
				Config: map[string]any{
					"User":     "testuser",
					"IP":       "localhost",
					"Port":     22,
					"Password": "testpassword",
				},
			},
		}
		ctx := context.Background()
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		// Type assert to StepValidator
		validator, ok := exec.(scheduler.StepValidator)
		require.True(t, ok, "SSH executor should implement StepValidator")

		// Should fail with script field
		err = validator.ValidateStep(&step)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "script field is not supported")
		assert.Contains(t, err.Error(), "Use 'command' field instead")
	})

	t.Run("ValidateStepWithCommand", func(t *testing.T) {
		step := digraph.Step{
			Name:    "ssh-with-command",
			Command: "echo 'test'",
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "ssh",
				Config: map[string]any{
					"User":     "testuser",
					"IP":       "localhost",
					"Port":     22,
					"Password": "testpassword",
				},
			},
		}
		ctx := context.Background()
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		// Type assert to StepValidator
		validator, ok := exec.(scheduler.StepValidator)
		require.True(t, ok, "SSH executor should implement StepValidator")

		// Should pass with command field
		err = validator.ValidateStep(&step)
		assert.NoError(t, err)
	})
}
