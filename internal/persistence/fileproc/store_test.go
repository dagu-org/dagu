package fileproc

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	store := New(baseDir)

	// Create a dagRun reference
	dagRun := digraph.DAGRunRef{
		Name: "test_dag",
		ID:   "test_id",
	}

	// Get the process for the dag-run
	// Using different group name (queue) vs dag name to test hierarchy
	proc, err := store.Acquire(ctx, "test_queue", dagRun)
	require.NoError(t, err, "failed to get proc")

	// Stop the process after a short delay
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 100) // Give some time for the file to be created
		err := proc.Stop(ctx)
		require.NoError(t, err, "failed to stop proc")
		close(done)
	}()

	// Give time for the heartbeat to start
	time.Sleep(time.Millisecond * 50)

	// Check if the count is 1
	count, err := store.CountAlive(ctx, "test_queue")
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Wait for the process to stop
	<-done

	// Check if the count is 0
	count, err = store.CountAlive(ctx, "test_queue")
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestStore_IsRunAlive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseDir := t.TempDir()
	store := New(baseDir)

	t.Run("NoProcessFile", func(t *testing.T) {
		dagRun := digraph.DAGRunRef{
			Name: "test-dag",
			ID:   "run-123",
		}

		// Test when no process file exists
		alive, err := store.IsRunAlive(ctx, "queue-1", dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("AliveProcess", func(t *testing.T) {
		dagRun := digraph.DAGRunRef{
			Name: "test-dag",
			ID:   "run-456",
		}

		// Create a process and start heartbeat
		// Use different group name (queue-2) vs dag name (test-dag)
		proc, err := store.Acquire(ctx, "queue-2", dagRun)
		require.NoError(t, err)

		// Give a moment for the heartbeat to start
		time.Sleep(time.Millisecond * 50)

		// Check if the run is alive
		alive, err := store.IsRunAlive(ctx, "queue-2", dagRun)
		require.NoError(t, err)
		require.True(t, alive)

		// Stop the process
		err = proc.Stop(ctx)
		require.NoError(t, err)

		// Check again - should be false now
		alive, err = store.IsRunAlive(ctx, "queue-2", dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("DifferentRunID", func(t *testing.T) {
		// Create a process for one run ID
		dagRun1 := digraph.DAGRunRef{
			Name: "test-dag-3",
			ID:   "run-789",
		}
		proc1, err := store.Acquire(ctx, "queue-3", dagRun1)
		require.NoError(t, err)

		// Give a moment for the heartbeat to start
		time.Sleep(time.Millisecond * 50)

		// Check for a different run ID
		dagRun2 := digraph.DAGRunRef{
			Name: "test-dag-3",
			ID:   "run-999",
		}
		alive, err := store.IsRunAlive(ctx, "queue-3", dagRun2)
		require.NoError(t, err)
		require.False(t, alive)

		// Check the original run is still alive
		alive, err = store.IsRunAlive(ctx, "queue-3", dagRun1)
		require.NoError(t, err)
		require.True(t, alive)

		// Cleanup
		err = proc1.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("StaleProcess", func(t *testing.T) {
		// Create a store with very short stale time for testing
		shortStore := &Store{
			baseDir:   baseDir,
			staleTime: time.Millisecond * 100,
		}

		dagRun := digraph.DAGRunRef{
			Name: "test-dag-stale",
			ID:   "run-stale",
		}

		// Create a process
		// Use different group name vs dag name
		proc, err := shortStore.Acquire(ctx, "stale-queue", dagRun)
		require.NoError(t, err)

		// Stop the heartbeat immediately
		err = proc.Stop(ctx)
		require.NoError(t, err)

		// Check if the run is alive (should become false when stale)
		require.Eventually(t, func() bool {
			alive, err := shortStore.IsRunAlive(ctx, "stale-queue", dagRun)
			return err == nil && !alive
		}, 200*time.Millisecond, 10*time.Millisecond, "expected process to become stale")
	})
}
