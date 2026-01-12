package filedagrun

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONDB(t *testing.T) {
	t.Run("RecentRecords", func(t *testing.T) {
		th := setupTestStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Running)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Failed)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Request 2 most recent attempts
		attempts := th.Store.RecentAttempts(th.Context, "test_DAG", 2)
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
		attempts = th.Store.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		attempts = th.Store.RecentAttempts(th.Context, "test_DAG", 4)
		require.Len(t, attempts, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Running)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Failed)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Set the database to return the latest status (even if it was created today)
		// Verify that record created before today is returned
		obj := th.Store.(*Store)
		obj.latestStatusToday = false
		attempt, err := th.Store.LatestAttempt(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		dagRunStatus, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "dagrun-id-3", dagRunStatus.DAGRunID)
	})
	t.Run("FindByDAGRunID", func(t *testing.T) {
		th := setupTestStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Running)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Failed)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Find the record with dag-run ID "dagrun-id-2"
		ref := exec.NewDAGRunRef("test_DAG", "dagrun-id-2")
		attempt, err := th.Store.FindAttempt(th.Context, ref)
		require.NoError(t, err)

		// Verify the record is the correct one
		dagRunStatus, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-2", dagRunStatus.DAGRunID)

		// Verify an error is returned if the dag-run ID does not exist
		refNonExist := exec.NewDAGRunRef("test_DAG", "nonexistent-id")
		_, err = th.Store.FindAttempt(th.Context, refNonExist)
		assert.ErrorIs(t, err, exec.ErrDAGRunIDNotFound)
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Running)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Failed)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Verify attempts are present
		attempts := th.Store.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 3)

		// Remove records older than 0 days
		// It should remove all records
		removedIDs, err := th.Store.RemoveOldDAGRuns(th.Context, "test_DAG", 0)
		require.NoError(t, err)
		assert.Len(t, removedIDs, 2) // 2 non-active runs should be removed

		// Verify non active attempts are removed
		attempts = th.Store.RecentAttempts(th.Context, "test_DAG", 3)
		require.Len(t, attempts, 1)

		// Verify the remaining attempt is the active one
		dagRunStatus, err := attempts[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "dagrun-id-1", dagRunStatus.DAGRunID)
		assert.Equal(t, core.Running, dagRunStatus.Status)
	})
	t.Run("SubDAGRun", func(t *testing.T) {
		th := setupTestStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateAttempt(t, ts, "parent-id", core.Running)

		// Create a child attempt
		rootDAGRun := exec.NewDAGRunRef("test_DAG", "parent-id")
		subDAG := th.DAG("child")
		subAttempt, err := th.Store.CreateAttempt(th.Context, subDAG.DAG, ts, "sub-id", exec.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
		})
		require.NoError(t, err)

		// Write the status
		err = subAttempt.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = subAttempt.Close(th.Context)
		}()

		statusToWrite := exec.InitialStatus(subDAG.DAG)
		statusToWrite.DAGRunID = "sub-id"
		err = subAttempt.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify record is created
		dagRunRef := exec.NewDAGRunRef("test_DAG", "parent-id")
		existingAttempt, err := th.Store.FindSubAttempt(th.Context, dagRunRef, "sub-id")
		require.NoError(t, err)

		dagRunStatus, err := existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", dagRunStatus.DAGRunID)
	})
	t.Run("SubDAGRunRetry", func(t *testing.T) {
		th := setupTestStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateAttempt(t, ts, "parent-id", core.Running)

		// Create a sub dag-run
		const subDAGRunID = "sub-dagrun-id"
		const parentDAGRunID = "parent-id"

		rootDAGRun := exec.NewDAGRunRef("test_DAG", parentDAGRunID)
		subDAG := th.DAG("child")
		attempt, err := th.Store.CreateAttempt(th.Context, subDAG.DAG, ts, subDAGRunID, exec.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
		})
		require.NoError(t, err)

		// Write the status
		err = attempt.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = attempt.Close(th.Context)
		}()

		statusToWrite := exec.InitialStatus(subDAG.DAG)
		statusToWrite.DAGRunID = subDAGRunID
		statusToWrite.Status = core.Running
		err = attempt.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Find the sub dag-run record
		ts = time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		dagRunRef := exec.NewDAGRunRef("test_DAG", parentDAGRunID)
		existingAttempt, err := th.Store.FindSubAttempt(th.Context, dagRunRef, subDAGRunID)
		require.NoError(t, err)
		existingAttemptStatus, err := existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, subDAGRunID, existingAttemptStatus.DAGRunID)
		assert.Equal(t, core.Running.String(), existingAttemptStatus.Status.String())

		// Create a retry record and write different status
		retryAttempt, err := th.Store.CreateAttempt(th.Context, subDAG.DAG, ts, subDAGRunID, exec.NewDAGRunAttemptOptions{
			RootDAGRun: &rootDAGRun,
			Retry:      true,
		})
		require.NoError(t, err)
		statusToWrite.Status = core.Succeeded
		_ = retryAttempt.Open(th.Context)
		_ = retryAttempt.Write(th.Context, statusToWrite)
		_ = retryAttempt.Close(th.Context)

		// Verify the retry record is created
		existingAttempt, err = th.Store.FindSubAttempt(th.Context, dagRunRef, subDAGRunID)
		require.NoError(t, err)
		existingAttemptStatus, err = existingAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, subDAGRunID, existingAttemptStatus.DAGRunID)
		assert.Equal(t, core.Succeeded.String(), existingAttemptStatus.Status.String())
	})
	t.Run("CreateSubAttempt", func(t *testing.T) {
		th := setupTestStore(t)

		// Create a parent record first
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		th.CreateAttempt(t, ts, "parent-id", core.Running)

		// Create sub-attempt using CreateSubAttempt
		rootRef := exec.NewDAGRunRef("test_DAG", "parent-id")
		subAttempt, err := th.Store.CreateSubAttempt(th.Context, rootRef, "sub-id")
		require.NoError(t, err)

		// Write status to the sub-attempt
		subDAG := th.DAG("child")
		subAttempt.SetDAG(subDAG.DAG)
		err = subAttempt.Open(th.Context)
		require.NoError(t, err)
		defer func() { _ = subAttempt.Close(th.Context) }()

		statusToWrite := exec.InitialStatus(subDAG.DAG)
		statusToWrite.DAGRunID = "sub-id"
		err = subAttempt.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify sub-attempt can be found
		foundAttempt, err := th.Store.FindSubAttempt(th.Context, rootRef, "sub-id")
		require.NoError(t, err)

		status, err := foundAttempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.DAGRunID)
	})
	t.Run("CreateSubAttemptEmptyRootID", func(t *testing.T) {
		th := setupTestStore(t)

		// Try to create sub-attempt with empty root ID
		rootRef := exec.NewDAGRunRef("test_DAG", "")
		_, err := th.Store.CreateSubAttempt(th.Context, rootRef, "sub-id")
		require.ErrorIs(t, err, ErrDAGRunIDEmpty)
	})
	t.Run("ReadDAG", func(t *testing.T) {
		th := setupTestStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		rec := th.CreateAttempt(t, ts, "parent-id", core.Running)

		// Write the status
		err := rec.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = rec.Close(th.Context)
		}()

		statusToWrite := exec.InitialStatus(rec.dag)
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

// TestListRootExactMatch verifies that listRoot does exact matching, not substring matching.
// Regression test for issue #1473.
func TestListRootExactMatch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "go"), 0750))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "go_fasthttp"), 0750))

	store := &Store{baseDir: tmpDir}
	roots, err := store.listRoot(context.Background(), "go")
	require.NoError(t, err)
	require.Len(t, roots, 1, "should only match 'go', not 'go_fasthttp'")
	assert.Equal(t, "go", roots[0].prefix)
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
		th := setupTestStore(t)

		// Create records with different timestamps
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Succeeded)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Succeeded)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Filter by time range (only ts2 should be included)
		from := exec.NewUTC(time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC))
		to := exec.NewUTC(time.Date(2021, 1, 2, 12, 0, 0, 0, time.UTC))

		statuses, err := th.Store.ListStatuses(th.Context,
			exec.WithFrom(from),
			exec.WithTo(to),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "dagrun-id-2", statuses[0].DAGRunID)
	})

	t.Run("FilterByStatus", func(t *testing.T) {
		th := setupTestStore(t)

		// Create records with different statuses
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		th.CreateAttempt(t, ts, "dagrun-id-1", core.Running)
		th.CreateAttempt(t, ts, "dagrun-id-2", core.Failed)
		th.CreateAttempt(t, ts, "dagrun-id-3", core.Succeeded)

		// Filter by status (only StatusError should be included)
		statuses, err := th.Store.ListStatuses(th.Context,
			exec.WithStatuses([]core.Status{core.Failed}),
			exec.WithFrom(exec.NewUTC(ts)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "dagrun-id-2", statuses[0].DAGRunID)
		assert.Equal(t, core.Failed, statuses[0].Status)
	})

	t.Run("LimitResults", func(t *testing.T) {
		th := setupTestStore(t)

		// Create multiple records
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 1; i <= 5; i++ {
			th.CreateAttempt(t, ts, fmt.Sprintf("dagrun-id-%d", i), core.Succeeded)
		}

		// Limit to 3 results
		options := &exec.ListDAGRunStatusesOptions{Limit: 3}
		statuses, err := th.Store.ListStatuses(th.Context, func(o *exec.ListDAGRunStatusesOptions) {
			o.Limit = options.Limit
		}, exec.WithFrom(exec.NewUTC(ts)))

		require.NoError(t, err)
		require.Len(t, statuses, 3)
	})

	t.Run("SortByCreatedAt", func(t *testing.T) {
		th := setupTestStore(t)

		// Use different timestamps to ensure deterministic sort order
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 1, 0, 0, 1, 0, time.UTC) // 1 second later
		ts3 := time.Date(2021, 1, 1, 0, 0, 2, 0, time.UTC) // 2 seconds later

		th.CreateAttempt(t, ts1, "dagrun-id-1", core.Succeeded)
		th.CreateAttempt(t, ts2, "dagrun-id-2", core.Succeeded)
		th.CreateAttempt(t, ts3, "dagrun-id-3", core.Succeeded)

		// Get all statuses
		statuses, err := th.Store.ListStatuses(
			th.Context, exec.WithFrom(exec.NewUTC(ts1)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 3)

		// Verify they are sorted by StartedAt in descending order (newest first)
		assert.Equal(t, "dagrun-id-3", statuses[0].DAGRunID)
		assert.Equal(t, "dagrun-id-2", statuses[1].DAGRunID)
		assert.Equal(t, "dagrun-id-1", statuses[2].DAGRunID)
	})
}

func TestLatestStatusTimezone(t *testing.T) {
	t.Run("LatestStatusTodayTimezoneIssue", func(t *testing.T) {
		// Simulate Europe/Paris timezone (UTC+2 in summer, UTC+1 in winter)
		parisLoc, err := time.LoadLocation("Europe/Paris")
		require.NoError(t, err)

		// Create a test store with Paris timezone
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		store := New(tmpDir,
			WithLatestStatusToday(true),
			WithLocation(parisLoc),
		)

		th := StoreTest{
			Context: context.Background(),
			Store:   store,
			TmpDir:  tmpDir,
		}

		// Create a DAG run at 00:00 Paris time on June 8, 2025
		// This is 22:00 UTC on June 7, 2025 (during DST, Paris is UTC+2)
		parisTime := time.Date(2025, 6, 8, 0, 0, 0, 0, parisLoc)
		utcTime := parisTime.UTC()

		// Verify our assumption about the time conversion
		assert.Equal(t, "2025-06-07 22:00:00 +0000 UTC", utcTime.String())

		// Create the DAG run at 00:00 Paris time
		th.CreateAttempt(t, utcTime, "midnight-run", core.Succeeded)

		// Simulate checking the status on June 8, 2025 at 10:00 UTC
		// (which is 12:00 Paris time on the same day)
		// The bug is that LatestAttempt uses time.Now() without considering the configured timezone
		// It will think "today" is June 8 in server time, but the run was at June 7 22:00 UTC
		// So it won't find the run that happened at 00:00 Paris time (June 7 22:00 UTC)

		// To simulate this, we'd need to mock time.Now(), but we can demonstrate the issue
		// by showing that when we look for runs "today" using UTC, we miss the Paris midnight run

		// With the fix, when we look for "today's" runs using Paris timezone,
		// it should find the run that happened at 00:00 Paris time (22:00 UTC previous day)
		// because it's "today" in Paris timezone.

		// To properly test this, we'd need to mock time.Now() to be on June 8, 2025
		// For now, let's verify that the timezone is properly set in the store
		obj := th.Store.(*Store)
		assert.Equal(t, parisLoc, obj.location)
		assert.True(t, obj.latestStatusToday)

		// Verify the run exists when checking without latestStatusToday
		obj.latestStatusToday = false
		attempt, err := th.Store.LatestAttempt(th.Context, "test_DAG")
		require.NoError(t, err)

		dagRunStatus, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "midnight-run", dagRunStatus.DAGRunID)
	})

	t.Run("LatestStatusTodayVerifyFix", func(t *testing.T) {
		// This test verifies that when we create runs at different times,
		// the "today" calculation uses the configured timezone correctly

		// Use Asia/Tokyo timezone (UTC+9)
		tokyoLoc, err := time.LoadLocation("Asia/Tokyo")
		require.NoError(t, err)

		// Create a test store with Tokyo timezone
		tmpDir, err := os.MkdirTemp("", "test")
		require.NoError(t, err)
		t.Cleanup(func() {
			_ = os.RemoveAll(tmpDir)
		})

		store := New(tmpDir,
			WithLatestStatusToday(true),
			WithLocation(tokyoLoc),
		)

		th := StoreTest{
			Context: context.Background(),
			Store:   store,
			TmpDir:  tmpDir,
		}

		// Create a run "today" in the configured timezone
		now := time.Now().In(tokyoLoc)
		todayInTokyo := time.Date(now.Year(), now.Month(), now.Day(), 1, 0, 0, 0, tokyoLoc)

		th.CreateAttempt(t, todayInTokyo, "tokyo-today-run", core.Succeeded)

		// This should find the run because it's "today" in Tokyo timezone
		attempt, err := th.Store.LatestAttempt(th.Context, "test_DAG")
		require.NoError(t, err)

		dagRunStatus, err := attempt.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "tokyo-today-run", dagRunStatus.DAGRunID)
	})
}
