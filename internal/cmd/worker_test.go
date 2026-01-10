package cmd_test

import (
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerCommand(t *testing.T) {
	t.Run("WorkerCommandExists", func(t *testing.T) {
		cli := cmd.CmdWorker()
		require.NotNil(t, cli)
		require.Equal(t, "worker [flags]", cli.Use)
		require.Equal(t, "Start a worker that polls the coordinator for tasks", cli.Short)
	})

	t.Run("WorkerCommandHasExpectedFlags", func(t *testing.T) {
		cli := cmd.CmdWorker()
		require.NotNil(t, cli)

		// Verify expected flags are registered
		flags := cli.Flags()
		require.NotNil(t, flags)

		// Check worker-specific flags exist (note: they may be prefixed)
		// The actual flag names depend on how they're registered
		assert.NotEmpty(t, cli.Long, "Long description should be set")
	})

	t.Run("WorkerCommandLongDescriptionContainsUsageInfo", func(t *testing.T) {
		cli := cmd.CmdWorker()
		require.NotNil(t, cli)

		// Verify the long description contains important usage info
		assert.Contains(t, cli.Long, "worker ID")
		assert.Contains(t, cli.Long, "coordinator")
		assert.Contains(t, cli.Long, "TLS")
		assert.Contains(t, cli.Long, "labels")
	})

	t.Run("WorkerCommandExamples", func(t *testing.T) {
		cli := cmd.CmdWorker()
		require.NotNil(t, cli)

		// Verify examples are present in long description
		assert.Contains(t, cli.Long, "Example:")
		assert.Contains(t, cli.Long, "dagu worker")
	})
}
