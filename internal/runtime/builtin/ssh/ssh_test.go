package ssh

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
