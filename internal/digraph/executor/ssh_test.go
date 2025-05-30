package executor

import (
	"context"
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
}
