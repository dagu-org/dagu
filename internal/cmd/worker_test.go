package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/stretchr/testify/require"
)

func TestWorkerCommand(t *testing.T) {
	t.Run("WorkerCommandExists", func(t *testing.T) {
		cli := cmd.CmdWorker()
		require.NotNil(t, cli)
		require.Equal(t, "worker [flags]", cli.Use)
		require.Equal(t, "Start a worker that polls the coordinator for tasks", cli.Short)
	})
}
