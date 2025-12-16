package filedagrun

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPrefixMatchingBug_Issue1473 reproduces the bug reported in GitHub issue #1473
// where DAGs with the same prefix (e.g., "go" and "go_fasthttp", or "dummy-go" and "dummy-go_fasthttp")
// can have their DAG-run data mixed up when searching for the latest run.
//
// The bug is in listRoot() function which uses strings.Contains() for filtering,
// causing "go" to match both "go" and "go_fasthttp" directories.
func TestPrefixMatchingBug_Issue1473(t *testing.T) {
	t.Run("ListStatuses_WithName_ShouldOnlyMatchExactDAGName", func(t *testing.T) {
		// Setup: Create a store with two DAGs that share a prefix
		tmpDir, err := os.MkdirTemp("", "prefix-bug-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		store := New(tmpDir, WithLatestStatusToday(false))
		ctx := context.Background()

		// Create DAG "go"
		dagGo := &core.DAG{
			Name:     "go",
			Location: filepath.Join(tmpDir, "go.yaml"),
		}

		// Create DAG "go_fasthttp" (which has "go" as prefix)
		dagGoFasthttp := &core.DAG{
			Name:     "go_fasthttp",
			Location: filepath.Join(tmpDir, "go_fasthttp.yaml"),
		}

		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a DAG run for "go"
		attemptGo, err := store.CreateAttempt(ctx, dagGo, ts, "go-dagrun-id", execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptGo.Open(ctx))
		statusGo := execution.InitialStatus(dagGo)
		statusGo.DAGRunID = "go-dagrun-id"
		statusGo.Status = core.Running
		require.NoError(t, attemptGo.Write(ctx, statusGo))
		require.NoError(t, attemptGo.Close(ctx))

		// Create a DAG run for "go_fasthttp"
		attemptGoFasthttp, err := store.CreateAttempt(ctx, dagGoFasthttp, ts.Add(time.Hour), "go_fasthttp-dagrun-id", execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptGoFasthttp.Open(ctx))
		statusGoFasthttp := execution.InitialStatus(dagGoFasthttp)
		statusGoFasthttp.DAGRunID = "go_fasthttp-dagrun-id"
		statusGoFasthttp.Status = core.Running
		require.NoError(t, attemptGoFasthttp.Write(ctx, statusGoFasthttp))
		require.NoError(t, attemptGoFasthttp.Close(ctx))

		// When we search for DAG "go" using WithName, it should ONLY return "go" results
		// NOT "go_fasthttp" results
		statuses, err := store.ListStatuses(ctx,
			execution.WithName("go"),
			execution.WithFrom(execution.NewUTC(ts)),
		)
		require.NoError(t, err)

		// CORRECT BEHAVIOR: Should only find 1 status for DAG "go"
		// BUG: Currently finds 2 statuses (both "go" and "go_fasthttp")
		require.Len(t, statuses, 1, "WithName('go') should only return DAG runs for 'go', not 'go_fasthttp'")
		assert.Equal(t, "go", statuses[0].Name, "Should only find 'go' DAG")
		assert.Equal(t, "go-dagrun-id", statuses[0].DAGRunID)
	})

	t.Run("ListStatuses_WithExactName_WorksCorrectly", func(t *testing.T) {
		// This test verifies that WithExactName works correctly (no prefix matching bug)
		tmpDir, err := os.MkdirTemp("", "prefix-bug-exact-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		store := New(tmpDir, WithLatestStatusToday(false))
		ctx := context.Background()

		// Create DAG "go"
		dagGo := &core.DAG{
			Name:     "go",
			Location: filepath.Join(tmpDir, "go.yaml"),
		}

		// Create DAG "go_fasthttp"
		dagGoFasthttp := &core.DAG{
			Name:     "go_fasthttp",
			Location: filepath.Join(tmpDir, "go_fasthttp.yaml"),
		}

		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create DAG runs for both
		attemptGo, err := store.CreateAttempt(ctx, dagGo, ts, "go-dagrun-id", execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptGo.Open(ctx))
		statusGo := execution.InitialStatus(dagGo)
		statusGo.DAGRunID = "go-dagrun-id"
		statusGo.Status = core.Running
		require.NoError(t, attemptGo.Write(ctx, statusGo))
		require.NoError(t, attemptGo.Close(ctx))

		attemptGoFasthttp, err := store.CreateAttempt(ctx, dagGoFasthttp, ts.Add(time.Hour), "go_fasthttp-dagrun-id", execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptGoFasthttp.Open(ctx))
		statusGoFasthttp := execution.InitialStatus(dagGoFasthttp)
		statusGoFasthttp.DAGRunID = "go_fasthttp-dagrun-id"
		statusGoFasthttp.Status = core.Running
		require.NoError(t, attemptGoFasthttp.Write(ctx, statusGoFasthttp))
		require.NoError(t, attemptGoFasthttp.Close(ctx))

		// Using WithExactName should NOT have the prefix matching bug
		statuses, err := store.ListStatuses(ctx,
			execution.WithExactName("go"),
			execution.WithFrom(execution.NewUTC(ts)),
		)
		require.NoError(t, err)

		// WithExactName should only return the exact match
		require.Len(t, statuses, 1, "WithExactName should only return exact matches")
		assert.Equal(t, "go", statuses[0].Name)
		assert.Equal(t, "go-dagrun-id", statuses[0].DAGRunID)
	})

	t.Run("listRoot_ShouldNotMatchSubstrings", func(t *testing.T) {
		// This test directly tests the listRoot function to verify correct behavior
		tmpDir, err := os.MkdirTemp("", "listroot-bug-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		// Create directories that simulate DAG data roots
		testDirs := []string{
			"go",          // Target - should match
			"go_fasthttp", // Should NOT match "go"
			"rust",        // Should NOT match "go"
		}

		for _, dir := range testDirs {
			dirPath := filepath.Join(tmpDir, dir)
			err := os.MkdirAll(dirPath, 0750)
			require.NoError(t, err)
		}

		store := &Store{baseDir: tmpDir}
		ctx := context.Background()

		// Call listRoot with "go" filter - should only match "go" exactly
		roots, err := store.listRoot(ctx, "go")
		require.NoError(t, err)

		// CORRECT BEHAVIOR: Should only return 1 directory ("go")
		// BUG: Currently returns 2 directories ("go" and "go_fasthttp")
		require.Len(t, roots, 1, "listRoot('go') should only match 'go' directory, not 'go_fasthttp'")
		assert.Equal(t, "go", roots[0].prefix)
	})

	t.Run("RealWorldScenario_DummyGo_DummyGoFasthttp", func(t *testing.T) {
		// Reproduces the exact scenario from issue #1473
		tmpDir, err := os.MkdirTemp("", "issue1473-test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		store := New(tmpDir, WithLatestStatusToday(false))
		ctx := context.Background()

		// Create the two DAGs from the bug report
		dagDummyGo := &core.DAG{
			Name:     "dummy-go",
			Location: filepath.Join(tmpDir, "dummy-go.yaml"),
		}

		dagDummyGoFasthttp := &core.DAG{
			Name:     "dummy-go_fasthttp",
			Location: filepath.Join(tmpDir, "dummy-go_fasthttp.yaml"),
		}

		ts := time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC)
		// Use a unique dag run ID (similar to UUID from the bug report)
		sharedStyleRunID := "e79b6918-9c57-49a8-a26b-df7de9fe6cd6"

		// Create a DAG run for "dummy-go" with Running status
		attemptDummyGo, err := store.CreateAttempt(ctx, dagDummyGo, ts, "dummy-go-run-1", execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptDummyGo.Open(ctx))
		statusDummyGo := execution.InitialStatus(dagDummyGo)
		statusDummyGo.DAGRunID = "dummy-go-run-1"
		statusDummyGo.Status = core.Running
		require.NoError(t, attemptDummyGo.Write(ctx, statusDummyGo))
		require.NoError(t, attemptDummyGo.Close(ctx))

		// Create a DAG run for "dummy-go_fasthttp" with Running status
		attemptDummyGoFasthttp, err := store.CreateAttempt(ctx, dagDummyGoFasthttp, ts.Add(time.Second), sharedStyleRunID, execution.NewDAGRunAttemptOptions{})
		require.NoError(t, err)
		require.NoError(t, attemptDummyGoFasthttp.Open(ctx))
		statusDummyGoFasthttp := execution.InitialStatus(dagDummyGoFasthttp)
		statusDummyGoFasthttp.DAGRunID = sharedStyleRunID
		statusDummyGoFasthttp.Status = core.Running
		require.NoError(t, attemptDummyGoFasthttp.Write(ctx, statusDummyGoFasthttp))
		require.NoError(t, attemptDummyGoFasthttp.Close(ctx))

		// Search for "dummy-go" DAG with Running status (simulating GetLatestStatus behavior)
		statuses, err := store.ListStatuses(ctx,
			execution.WithName("dummy-go"),
			execution.WithStatuses([]core.Status{core.Running}),
			execution.WithFrom(execution.NewUTC(ts)),
		)
		require.NoError(t, err)

		// CORRECT BEHAVIOR: Should only find 1 status for "dummy-go"
		// BUG: Currently finds 2 statuses (both "dummy-go" and "dummy-go_fasthttp")
		// This is the root cause of issue #1473
		require.Len(t, statuses, 1, "Searching for 'dummy-go' should not return 'dummy-go_fasthttp' results")
		assert.Equal(t, "dummy-go", statuses[0].Name, "Should only find 'dummy-go' DAG")
		assert.Equal(t, "dummy-go-run-1", statuses[0].DAGRunID)
	})
}
