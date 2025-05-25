package localhistory

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONDB(t *testing.T) {
	t.Run("RecentRecords", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusRunning)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusError)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Request 2 most recent attempts
		attempts := th.DAGRunStore.RecentAttempts(th.Context, "test_DAG", 2)
		require.Len(t, attempts, 2)

		// Verify the first record is the most recent
		status0, err := attempts[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-3", status0.DAGRunID)

		// Verify the second record is the second most recent
		status1, err := attempts[1].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-2", status1.DAGRunID)

		// Verify all records are returned if the number requested is equal to the number of records
		attempts = th.DAGRunStore.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		attempts = th.DAGRunStore.RecentAttempts(th.Context, "test_DAG", 4)
		require.Len(t, attempts, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusRunning)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusError)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Set the database to return the latest status (even if it was created today)
		// Verify that record created before today is returned
		obj := th.DAGRunStore.(*Store)
		obj.latestStatusToday = false
		attempt, err := th.DAGRunStore.LatestAttempt(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "dagrun-id-3", status.DAGRunID)
	})
	t.Run("FindByDAGRunID", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusRunning)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusError)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Find the record with DAG-run ID "dagrun-id-2"
		ref := digraph.NewDAGRunRef("test_DAG", "dagrun-id-2")
		attempt, err := th.DAGRunStore.FindAttempt(th.Context, ref)
		require.NoError(t, err)

		// Verify the record is the correct one
		status, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-2", status.DAGRunID)

		// Verify an error is returned if the DAG-run ID does not exist
		refNonExist := digraph.NewDAGRunRef("test_DAG", "nonexistent-id")
		_, err = th.DAGRunStore.FindAttempt(th.Context, refNonExist)
		assert.ErrorIs(t, err, models.ErrDAGRunIDNotFound)
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusRunning)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusError)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Verify attempts are present
		attempts := th.DAGRunStore.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 3)

		// Remove records older than 0 days
		// It should remove all records
		err := th.DAGRunStore.RemoveOldDAGRuns(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		// Verify non active attempts are removed
		attempts = th.DAGRunStore.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 1)

		// Verify the remaining attempt is the active one
		status, err := attempts[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-1", status.DAGRunID)
		assert.Equal(t, scheduler.StatusRunning, status.Status)
	})
	t.Run("ChildDAGRun", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateAttempt(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child attempt
		rootDAGRun := digraph.NewDAGRunRef("test_DAG", "parent-id")
		childDAG := th.DAG("child")
		childAttempt, err := th.DAGRunStore.CreateAttempt(th.Context, childDAG.DAG, ts, "sub-id", models.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
		})
		require.NoError(t, err)

		// Write the status
		err = childAttempt.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = childAttempt.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(childDAG.DAG)
		statusToWrite.DAGRunID = "sub-id"
		err = childAttempt.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify record is created
		dagRunRef := digraph.NewDAGRunRef("test_DAG", "parent-id")
		existingAttempt, err := th.DAGRunStore.FindChildAttempt(th.Context, dagRunRef, "sub-id")
		require.NoError(t, err)

		status, err := existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.DAGRunID)
	})
	t.Run("ChildDAGRun_Retry", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateAttempt(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child DAG-run
		const childDAGRunID = "child-dagrun-id"
		const parentDAGRunID = "parent-id"

		rootDAGRun := digraph.NewDAGRunRef("test_DAG", parentDAGRunID)
		childDAG := th.DAG("child")
		attempt, err := th.DAGRunStore.CreateAttempt(th.Context, childDAG.DAG, ts, childDAGRunID, models.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
		})
		require.NoError(t, err)

		// Write the status
		err = attempt.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = attempt.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(childDAG.DAG)
		statusToWrite.DAGRunID = childDAGRunID
		statusToWrite.Status = scheduler.StatusRunning
		err = attempt.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Find the child DAG-run record
		ts = time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		dagRunRef := digraph.NewDAGRunRef("test_DAG", parentDAGRunID)
		existingAttempt, err := th.DAGRunStore.FindChildAttempt(th.Context, dagRunRef, childDAGRunID)
		require.NoError(t, err)
		existingAttemptStatus, err := existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childDAGRunID, existingAttemptStatus.DAGRunID)
		assert.Equal(t, scheduler.StatusRunning.String(), existingAttemptStatus.Status.String())

		// Create a retry record and write different status
		retryAttempt, err := th.DAGRunStore.CreateAttempt(th.Context, childDAG.DAG, ts, childDAGRunID, models.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
			Retry:      true,
		})
		require.NoError(t, err)
		statusToWrite.Status = scheduler.StatusSuccess
		_ = retryAttempt.Open(th.Context)
		_ = retryAttempt.Write(th.Context, statusToWrite)
		_ = retryAttempt.Close(th.Context)

		// Verify the retry record is created
		existingAttempt, err = th.DAGRunStore.FindChildAttempt(th.Context, dagRunRef, childDAGRunID)
		require.NoError(t, err)
		existingAttemptStatus, err = existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childDAGRunID, existingAttemptStatus.DAGRunID)
		assert.Equal(t, scheduler.StatusSuccess.String(), existingAttemptStatus.Status.String())
	})
	t.Run("ReadDAG", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		rec := th.CreateAttempt(t, ts, "parent-id", scheduler.StatusRunning)

		// Write the status
		err := rec.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = rec.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(rec.dag)
		statusToWrite.DAGRunID = "parent-id"

		err = rec.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Read the DAG and verify it matches the original
		dag, err := rec.ReadDAG(th.Context)
		require.NoError(t, err)

		require.NotNil(t, dag)
		require.Equal(t, *rec.dag, *dag)
	})
}

func TestListRoot(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create test directories
	testDirs := []string{
		"dag1",
		"dag2",
		"dag3",
	}

	for _, dir := range testDirs {
		dirPath := filepath.Join(tmpDir, dir)
		err := os.MkdirAll(dirPath, 0750)
		require.NoError(t, err, "Failed to create test directory")
	}

	// Create a file (should be ignored by listRoot)
	filePath := filepath.Join(tmpDir, "not-a-dir.txt")
	err := os.WriteFile(filePath, []byte("test"), 0600)
	require.NoError(t, err, "Failed to create test file")

	// Create localStore instance
	store := &Store{baseDir: tmpDir}

	// Call listRoot
	ctx := context.Background()
	roots, err := store.listRoot(ctx, "")
	require.NoError(t, err, "listRoot should not return an error")

	// Verify results
	assert.Len(t, roots, len(testDirs), "listRoot should return the correct number of directories")

	// Verify each directory is in the results
	foundDirs := make(map[string]bool)
	for _, root := range roots {
		foundDirs[root.prefix] = true
	}

	for _, dir := range testDirs {
		assert.True(t, foundDirs[dir], "listRoot should include directory %s", dir)
	}
}

func TestListRootEmptyDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create localStore instance
	store := &Store{baseDir: tmpDir}

	// Call listRoot
	ctx := context.Background()
	roots, err := store.listRoot(ctx, "")
	require.NoError(t, err, "listRoot should not return an error")

	// Verify results
	assert.Len(t, roots, 0, "listRoot should return an empty slice for an empty directory")
}

func TestListRootNonExistentDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()
	nonExistentDir := filepath.Join(tmpDir, "non-existent")

	// Create localStore instance
	store := &Store{baseDir: nonExistentDir}

	// Call listRoot
	ctx := context.Background()
	roots, err := store.listRoot(ctx, "")
	require.NoError(t, err, "listRoot should not return an error for non-existent directory")

	// Verify results
	assert.Len(t, roots, 0, "listRoot should return an empty slice for a non-existent directory")
}

func TestListRootCanceledContext(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create localStore instance
	store := &Store{baseDir: tmpDir}

	// Create a canceled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel the context immediately

	// Call listRoot with canceled context
	roots, err := store.listRoot(ctx, "")

	// The function doesn't check for context cancellation, so it should still succeed
	require.NoError(t, err, "listRoot should not return an error for canceled context")
	assert.Len(t, roots, 0, "listRoot should return an empty slice for an empty directory")
}

func TestListStatuses(t *testing.T) {
	t.Run("FilterByTimeRange", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create records with different timestamps
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusSuccess)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusSuccess)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Filter by time range (only ts2 should be included)
		from := models.NewUTC(time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC))
		to := models.NewUTC(time.Date(2021, 1, 2, 12, 0, 0, 0, time.UTC))

		statuses, err := th.DAGRunStore.ListStatuses(th.Context,
			models.WithFrom(from),
			models.WithTo(to),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "dagrun-id-2", statuses[0].DAGRunID)
	})

	t.Run("FilterByStatus", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create records with different statuses
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		th.CreateAttempt(t, ts, "dagrun-id-1", scheduler.StatusRunning)
		th.CreateAttempt(t, ts, "dagrun-id-2", scheduler.StatusError)
		th.CreateAttempt(t, ts, "dagrun-id-3", scheduler.StatusSuccess)

		// Filter by status (only StatusError should be included)
		statuses, err := th.DAGRunStore.ListStatuses(th.Context,
			models.WithStatuses([]scheduler.Status{scheduler.StatusError}),
			models.WithFrom(models.NewUTC(ts)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "dagrun-id-2", statuses[0].DAGRunID)
		assert.Equal(t, scheduler.StatusError, statuses[0].Status)
	})

	t.Run("LimitResults", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create multiple records
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 1; i <= 5; i++ {
			th.CreateAttempt(t, ts, fmt.Sprintf("dagrun-id-%d", i), scheduler.StatusSuccess)
		}

		// Limit to 3 results
		options := &models.ListDAGRunStatusesOptions{Limit: 3}
		statuses, err := th.DAGRunStore.ListStatuses(th.Context, func(o *models.ListDAGRunStatusesOptions) {
			o.Limit = options.Limit
		}, models.WithFrom(models.NewUTC(ts)))

		require.NoError(t, err)
		require.Len(t, statuses, 3)
	})

	t.Run("SortByCreatedAt", func(t *testing.T) {
		th := setupTestLocalStore(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		th.CreateAttempt(t, ts1, "dagrun-id-1", scheduler.StatusSuccess)
		th.CreateAttempt(t, ts2, "dagrun-id-2", scheduler.StatusSuccess)
		th.CreateAttempt(t, ts3, "dagrun-id-3", scheduler.StatusSuccess)

		// Get all statuses
		statuses, err := th.DAGRunStore.ListStatuses(
			th.Context, models.WithFrom(models.NewUTC(ts1)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 3)

		// Verify they are sorted by StartedAt in descending order
		assert.Equal(t, "dagrun-id-3", statuses[0].DAGRunID)
		assert.Equal(t, "dagrun-id-2", statuses[1].DAGRunID)
		assert.Equal(t, "dagrun-id-1", statuses[2].DAGRunID)
	})
}
