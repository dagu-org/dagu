package executor

import (
	"context"
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
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
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		sshExec, ok := exec.(*sshExec)
		require.True(t, ok)

		assert.Equal(t, "testuser", sshExec.config.User)
		assert.Equal(t, "testip", sshExec.config.IP)
		assert.Equal(t, "25", sshExec.config.Port)
		assert.Equal(t, "testpassword", sshExec.config.Password)
	})

	t.Run("ExpandEnv", func(t *testing.T) {
		os.Setenv("TEST_SSH_EXEC_USER", "testuser")
		os.Setenv("TEST_SSH_EXEC_IP", "testip")
		os.Setenv("TEST_SSH_EXEC_PORT", "23")
		os.Setenv("TEST_SSH_EXEC_PASSWORD", "testpassword")

		step := digraph.Step{
			Name: "ssh-exec",
			ExecutorConfig: digraph.ExecutorConfig{
				Type: "ssh",
				Config: map[string]any{
					"User":     "${TEST_SSH_EXEC_USER}",
					"IP":       "${TEST_SSH_EXEC_IP}",
					"Port":     "${TEST_SSH_EXEC_PORT}",
					"Password": "${TEST_SSH_EXEC_PASSWORD}",
				},
			},
		}
		ctx := context.Background()
		exec, err := newSSHExec(ctx, step)
		require.NoError(t, err)

		sshExec, ok := exec.(*sshExec)
		require.True(t, ok)

		assert.Equal(t, "testuser", sshExec.config.User)
		assert.Equal(t, "testip", sshExec.config.IP)
		assert.Equal(t, "23", sshExec.config.Port)
		assert.Equal(t, "testpassword", sshExec.config.Password)
	})
}
