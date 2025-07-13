package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/stretchr/testify/require"
)

func TestWorkerCommand(t *testing.T) {
	t.Run("WorkerCommandExists", func(t *testing.T) {
		cmd := cmd.CmdWorker()
		require.NotNil(t, cmd)
		require.Equal(t, "worker [flags]", cmd.Use)
		require.Equal(t, "Start a worker that polls the coordinator for tasks", cmd.Short)
	})
}
