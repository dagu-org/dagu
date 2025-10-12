package fileproc

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	store := New(baseDir)

	// Create a dagRun reference
	dagRun := execution.DAGRunRef{
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
		dagRun := execution.DAGRunRef{
			Name: "test-dag",
			ID:   "run-123",
		}

		// Test when no process file exists
		alive, err := store.IsRunAlive(ctx, "queue-1", dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("AliveProcess", func(t *testing.T) {
		dagRun := execution.DAGRunRef{
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
		dagRun1 := execution.DAGRunRef{
			Name: "test-dag-3",
			ID:   "run-789",
		}
		proc1, err := store.Acquire(ctx, "queue-3", dagRun1)
		require.NoError(t, err)

		// Give a moment for the heartbeat to start
		time.Sleep(time.Millisecond * 50)

		// Check for a different run ID
		dagRun2 := execution.DAGRunRef{
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

		dagRun := execution.DAGRunRef{
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

func TestStore_ListAllAlive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseDir := t.TempDir()
	store := New(baseDir)

	t.Run("EmptyStore", func(t *testing.T) {
		// Test when no processes exist
		result, err := store.ListAllAlive(ctx)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, result)
	})

	t.Run("SingleGroup", func(t *testing.T) {
		// Create processes in a single group
		dagRun1 := execution.DAGRunRef{
			Name: "dag1",
			ID:   "run1",
		}
		dagRun2 := execution.DAGRunRef{
			Name: "dag2",
			ID:   "run2",
		}

		proc1, err := store.Acquire(ctx, "queue1", dagRun1)
		require.NoError(t, err)
		defer func() { _ = proc1.Stop(ctx) }()

		proc2, err := store.Acquire(ctx, "queue1", dagRun2)
		require.NoError(t, err)
		defer func() { _ = proc2.Stop(ctx) }()

		// Give time for heartbeats to start
		time.Sleep(time.Millisecond * 50)

		// List all alive processes
		result, err := store.ListAllAlive(ctx)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Contains(t, result, "queue1")
		require.Len(t, result["queue1"], 2)

		// Check that both DAG runs are in the result
		runIDs := make(map[string]bool)
		for _, ref := range result["queue1"] {
			runIDs[ref.ID] = true
		}
		require.True(t, runIDs["run1"])
		require.True(t, runIDs["run2"])
	})

	t.Run("MultipleGroups", func(t *testing.T) {
		// Create processes in multiple groups
		dagRun1 := execution.DAGRunRef{
			Name: "dag-a",
			ID:   "run-a1",
		}
		dagRun2 := execution.DAGRunRef{
			Name: "dag-b",
			ID:   "run-b1",
		}
		dagRun3 := execution.DAGRunRef{
			Name: "dag-c",
			ID:   "run-c1",
		}

		proc1, err := store.Acquire(ctx, "queue-alpha", dagRun1)
		require.NoError(t, err)
		defer func() { _ = proc1.Stop(ctx) }()

		proc2, err := store.Acquire(ctx, "queue-beta", dagRun2)
		require.NoError(t, err)
		defer func() { _ = proc2.Stop(ctx) }()

		proc3, err := store.Acquire(ctx, "queue-alpha", dagRun3)
		require.NoError(t, err)
		defer func() { _ = proc3.Stop(ctx) }()

		// Give time for heartbeats to start
		time.Sleep(time.Millisecond * 50)

		// List all alive processes
		result, err := store.ListAllAlive(ctx)
		require.NoError(t, err)
		require.Len(t, result, 2)
		require.Contains(t, result, "queue-alpha")
		require.Contains(t, result, "queue-beta")
		require.Len(t, result["queue-alpha"], 2)
		require.Len(t, result["queue-beta"], 1)

		// Verify specific runs
		require.Equal(t, "run-b1", result["queue-beta"][0].ID)
	})

	t.Run("MixedAliveAndStopped", func(t *testing.T) {
		// Create some processes and stop some
		dagRun1 := execution.DAGRunRef{
			Name: "dag-x",
			ID:   "run-x1",
		}
		dagRun2 := execution.DAGRunRef{
			Name: "dag-y",
			ID:   "run-y1",
		}
		dagRun3 := execution.DAGRunRef{
			Name: "dag-z",
			ID:   "run-z1",
		}

		proc1, err := store.Acquire(ctx, "mixed-queue", dagRun1)
		require.NoError(t, err)

		proc2, err := store.Acquire(ctx, "mixed-queue", dagRun2)
		require.NoError(t, err)

		proc3, err := store.Acquire(ctx, "mixed-queue", dagRun3)
		require.NoError(t, err)

		// Give time for heartbeats to start
		time.Sleep(time.Millisecond * 50)

		// Stop proc2
		err = proc2.Stop(ctx)
		require.NoError(t, err)

		// List all alive processes
		result, err := store.ListAllAlive(ctx)
		require.NoError(t, err)
		require.Len(t, result, 1)
		require.Contains(t, result, "mixed-queue")
		require.Len(t, result["mixed-queue"], 2) // Only proc1 and proc3 should be alive

		// Verify the stopped process is not in the result
		runIDs := make(map[string]bool)
		for _, ref := range result["mixed-queue"] {
			runIDs[ref.ID] = true
		}
		require.True(t, runIDs["run-x1"])
		require.False(t, runIDs["run-y1"]) // This one was stopped
		require.True(t, runIDs["run-z1"])

		// Cleanup
		err = proc1.Stop(ctx)
		require.NoError(t, err)
		err = proc3.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("NonExistentBaseDir", func(t *testing.T) {
		// Test with a base directory that doesn't exist
		nonExistentStore := New("/tmp/non-existent-dir-12345")
		result, err := nonExistentStore.ListAllAlive(ctx)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, result)
	})
}
