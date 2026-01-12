package fileproc

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	store := New(baseDir)

	// Create a dagRun reference
	dagRun := exec.DAGRunRef{
		Name: "test_dag",
		ID:   "test_id",
	}

	// Get the process for the dag-run
	// Using different group name (queue) vs dag name to test hierarchy
	proc, err := store.Acquire(ctx, "test_queue", dagRun)
	require.NoError(t, err, "failed to get proc")

	requireCountAlive(t, ctx, store, "test_queue", 1)

	err = proc.Stop(ctx)
	require.NoError(t, err, "failed to stop proc")

	requireCountAlive(t, ctx, store, "test_queue", 0)
}

func TestStore_IsRunAlive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseDir := t.TempDir()
	store := New(baseDir)

	t.Run("NoProcessFile", func(t *testing.T) {
		dagRun := exec.DAGRunRef{
			Name: "test-dag",
			ID:   "run-123",
		}

		// Test when no process file exists
		alive, err := store.IsRunAlive(ctx, "queue-1", dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("AliveProcess", func(t *testing.T) {
		dagRun := exec.DAGRunRef{
			Name: "test-dag",
			ID:   "run-456",
		}

		// Create a process and start heartbeat
		// Use different group name (queue-2) vs dag name (test-dag)
		proc, err := store.Acquire(ctx, "queue-2", dagRun)
		require.NoError(t, err)

		requireRunAliveState(t, ctx, store, "queue-2", dagRun, true)

		// Stop the process
		err = proc.Stop(ctx)
		require.NoError(t, err)

		requireRunAliveState(t, ctx, store, "queue-2", dagRun, false)
	})

	t.Run("DifferentRunID", func(t *testing.T) {
		// Create a process for one run ID
		dagRun1 := exec.DAGRunRef{
			Name: "test-dag-3",
			ID:   "run-789",
		}
		proc1, err := store.Acquire(ctx, "queue-3", dagRun1)
		require.NoError(t, err)

		requireRunAliveState(t, ctx, store, "queue-3", dagRun1, true)

		// Check for a different run ID
		dagRun2 := exec.DAGRunRef{
			Name: "test-dag-3",
			ID:   "run-999",
		}
		alive, err := store.IsRunAlive(ctx, "queue-3", dagRun2)
		require.NoError(t, err)
		require.False(t, alive)

		// Check the original run is still alive
		requireRunAliveState(t, ctx, store, "queue-3", dagRun1, true)

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

		dagRun := exec.DAGRunRef{
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
		dagRun1 := exec.DAGRunRef{
			Name: "dag1",
			ID:   "run1",
		}
		dagRun2 := exec.DAGRunRef{
			Name: "dag2",
			ID:   "run2",
		}

		proc1, err := store.Acquire(ctx, "queue1", dagRun1)
		require.NoError(t, err)
		defer func() { _ = proc1.Stop(ctx) }()

		proc2, err := store.Acquire(ctx, "queue1", dagRun2)
		require.NoError(t, err)
		defer func() { _ = proc2.Stop(ctx) }()

		requireListAllAlive(t, ctx, store, func(result map[string][]exec.DAGRunRef) bool {
			queueRuns, ok := result["queue1"]
			return ok && len(result) == 1 && len(queueRuns) == 2
		}, "expected queue1 to contain both runs")

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
		dagRun1 := exec.DAGRunRef{
			Name: "dag-a",
			ID:   "run-a1",
		}
		dagRun2 := exec.DAGRunRef{
			Name: "dag-b",
			ID:   "run-b1",
		}
		dagRun3 := exec.DAGRunRef{
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

		requireListAllAlive(t, ctx, store, func(result map[string][]exec.DAGRunRef) bool {
			queueAlpha, alphaOK := result["queue-alpha"]
			queueBeta, betaOK := result["queue-beta"]
			return len(result) == 2 && alphaOK && betaOK && len(queueAlpha) == 2 && len(queueBeta) == 1
		}, "expected both queues to be populated")

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
		dagRun1 := exec.DAGRunRef{
			Name: "dag-x",
			ID:   "run-x1",
		}
		dagRun2 := exec.DAGRunRef{
			Name: "dag-y",
			ID:   "run-y1",
		}
		dagRun3 := exec.DAGRunRef{
			Name: "dag-z",
			ID:   "run-z1",
		}

		proc1, err := store.Acquire(ctx, "mixed-queue", dagRun1)
		require.NoError(t, err)

		proc2, err := store.Acquire(ctx, "mixed-queue", dagRun2)
		require.NoError(t, err)

		proc3, err := store.Acquire(ctx, "mixed-queue", dagRun3)
		require.NoError(t, err)

		// Stop proc2
		err = proc2.Stop(ctx)
		require.NoError(t, err)

		requireListAllAlive(t, ctx, store, func(result map[string][]exec.DAGRunRef) bool {
			queueRuns, ok := result["mixed-queue"]
			if !ok || len(result) != 1 || len(queueRuns) != 2 {
				return false
			}

			hasRunX := false
			hasRunY := false
			hasRunZ := false
			for _, ref := range queueRuns {
				switch ref.ID {
				case "run-x1":
					hasRunX = true
				case "run-y1":
					hasRunY = true
				case "run-z1":
					hasRunZ = true
				}
			}

			return hasRunX && hasRunZ && !hasRunY
		}, "expected only run-x1 and run-z1 to be alive")

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

const (
	heartbeatWait = 2 * time.Second
	heartbeatPoll = 10 * time.Millisecond
)

func requireCountAlive(t *testing.T, ctx context.Context, store *Store, queue string, expected int) {
	t.Helper()
	message := fmt.Sprintf("expected %d proc file(s) in %s", expected, queue)
	require.Eventually(t, func() bool {
		count, err := store.CountAlive(ctx, queue)
		require.NoError(t, err, "failed to count proc files")
		return count == expected
	}, heartbeatWait, heartbeatPoll, message)
}

func requireRunAliveState(t *testing.T, ctx context.Context, store *Store, queue string, dagRun exec.DAGRunRef, expected bool) {
	t.Helper()
	message := fmt.Sprintf("expected run %s/%s alive=%t", dagRun.Name, dagRun.ID, expected)
	require.Eventually(t, func() bool {
		alive, err := store.IsRunAlive(ctx, queue, dagRun)
		require.NoError(t, err)
		return alive == expected
	}, heartbeatWait, heartbeatPoll, message)
}

func requireListAllAlive(t *testing.T, ctx context.Context, store *Store, predicate func(map[string][]exec.DAGRunRef) bool, message string) {
	t.Helper()
	require.Eventually(t, func() bool {
		result, err := store.ListAllAlive(ctx)
		require.NoError(t, err)
		return predicate(result)
	}, heartbeatWait, heartbeatPoll, message)
}
