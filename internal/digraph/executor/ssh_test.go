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
		_, err := newSSHExec(ctx, step)
		require.NoError(t, err)
	})

	t.Run("StdoutStderrSeparation", func(t *testing.T) {
		step := digraph.Step{
			Name:    "ssh-exec-streams",
			Command: "echo 'stdout message' && echo 'stderr message' >&2",
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

		sshExec, ok := exec.(*sshExec)
		require.True(t, ok)

		// Verify that stdout and stderr are different writers
		assert.NotNil(t, sshExec.stdout)
		assert.NotNil(t, sshExec.stderr)

		// Test SetStdout and SetStderr
		stdoutWriter := &testWriter{}
		stderrWriter := &testWriter{}

		exec.SetStdout(stdoutWriter)
		exec.SetStderr(stderrWriter)

		// Verify they were set to different fields
		assert.Equal(t, stdoutWriter, sshExec.stdout)
		assert.Equal(t, stderrWriter, sshExec.stderr)
		// Ensure they're different instances
		assert.NotSame(t, sshExec.stdout, sshExec.stderr)
	})
}
