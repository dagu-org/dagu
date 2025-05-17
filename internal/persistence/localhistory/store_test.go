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
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Request 2 most recent runs
		runs := th.HistoryStore.RecentRuns(th.Context, "test_DAG", 2)
		require.Len(t, runs, 2)

		// Verify the first record is the most recent
		status0, err := runs[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "workflow-id-3", status0.WorkflowID)

		// Verify the second record is the second most recent
		status1, err := runs[1].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "workflow-id-2", status1.WorkflowID)

		// Verify all records are returned if the number requested is equal to the number of records
		runs = th.HistoryStore.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		runs = th.HistoryStore.RecentRuns(th.Context, "test_DAG", 4)
		require.Len(t, runs, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Set the database to return the latest status (even if it was created today)
		// Verify that record created before today is returned
		obj := th.HistoryStore.(*Store)
		obj.latestStatusToday = false
		run, err := th.HistoryStore.LatestRun(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		status, err := run.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "workflow-id-3", status.WorkflowID)
	})
	t.Run("FindByWorkflowID", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Find the record with workflow ID "workflow-id-2"
		ref := digraph.NewWorkflowRef("test_DAG", "workflow-id-2")
		run, err := th.HistoryStore.FindRun(th.Context, ref)
		require.NoError(t, err)

		// Verify the record is the correct one
		status, err := run.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "workflow-id-2", status.WorkflowID)

		// Verify an error is returned if the workflow ID does not exist
		refNonExist := digraph.NewWorkflowRef("test_DAG", "nonexistent-id")
		_, err = th.HistoryStore.FindRun(th.Context, refNonExist)
		assert.ErrorIs(t, err, models.ErrWorkflowIDNotFound)
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Verify runs are present
		runs := th.HistoryStore.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 3)

		// Remove records older than 0 days
		// It should remove all records
		err := th.HistoryStore.RemoveOldWorkflows(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		// Verify records are removed
		runs = th.HistoryStore.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 0)
	})
	t.Run("ChildWorkflow", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRun(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child run
		root := digraph.NewWorkflowRef("test_DAG", "parent-id")
		childWorkflowDAG := th.DAG("child")
		childRun, err := th.HistoryStore.CreateRun(th.Context, childWorkflowDAG.DAG, ts, "sub-id", models.NewRunOptions{
			Root: &root,
		})
		require.NoError(t, err)

		// Write the status
		err = childRun.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = childRun.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(childWorkflowDAG.DAG)
		statusToWrite.WorkflowID = "sub-id"
		err = childRun.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify record is created
		workflowRef := digraph.NewWorkflowRef("test_DAG", "parent-id")
		existingRun, err := th.HistoryStore.FindChildWorkflowRun(th.Context, workflowRef, "sub-id")
		require.NoError(t, err)

		status, err := existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.WorkflowID)
	})
	t.Run("ChildWorkflow_Retry", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRun(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child workflow
		const childWorkflowID = "child-workflow-id"
		const parentExecID = "parent-id"

		root := digraph.NewWorkflowRef("test_DAG", parentExecID)
		childWorkflowDAG := th.DAG("child")
		run, err := th.HistoryStore.CreateRun(th.Context, childWorkflowDAG.DAG, ts, childWorkflowID, models.NewRunOptions{
			Root: &root,
		})
		require.NoError(t, err)

		// Write the status
		err = run.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = run.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(childWorkflowDAG.DAG)
		statusToWrite.WorkflowID = childWorkflowID
		statusToWrite.Status = scheduler.StatusRunning
		err = run.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Find the child workflow record
		ts = time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		workflowRef := digraph.NewWorkflowRef("test_DAG", parentExecID)
		existingRun, err := th.HistoryStore.FindChildWorkflowRun(th.Context, workflowRef, childWorkflowID)
		require.NoError(t, err)
		existingRunStatus, err := existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childWorkflowID, existingRunStatus.WorkflowID)
		assert.Equal(t, scheduler.StatusRunning.String(), existingRunStatus.Status.String())

		// Create a retry record and write different status
		retryRun, err := th.HistoryStore.CreateRun(th.Context, childWorkflowDAG.DAG, ts, childWorkflowID, models.NewRunOptions{
			Root:  &root,
			Retry: true,
		})
		require.NoError(t, err)
		statusToWrite.Status = scheduler.StatusSuccess
		_ = retryRun.Open(th.Context)
		_ = retryRun.Write(th.Context, statusToWrite)
		_ = retryRun.Close(th.Context)

		// Verify the retry record is created
		existingRun, err = th.HistoryStore.FindChildWorkflowRun(th.Context, workflowRef, childWorkflowID)
		require.NoError(t, err)
		existingRunStatus, err = existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childWorkflowID, existingRunStatus.WorkflowID)
		assert.Equal(t, scheduler.StatusSuccess.String(), existingRunStatus.Status.String())
	})
	t.Run("ReadDAG", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		rec := th.CreateRun(t, ts, "parent-id", scheduler.StatusRunning)

		// Write the status
		err := rec.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = rec.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(rec.dag)
		statusToWrite.WorkflowID = "parent-id"

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

		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusSuccess)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusSuccess)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Filter by time range (only ts2 should be included)
		from := models.NewUTC(time.Date(2021, 1, 1, 12, 0, 0, 0, time.UTC))
		to := models.NewUTC(time.Date(2021, 1, 2, 12, 0, 0, 0, time.UTC))

		statuses, err := th.HistoryStore.ListStatuses(th.Context,
			models.WithFrom(from),
			models.WithTo(to),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "workflow-id-2", statuses[0].WorkflowID)
	})

	t.Run("FilterByStatus", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create records with different statuses
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		th.CreateRun(t, ts, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts, "workflow-id-3", scheduler.StatusSuccess)

		// Filter by status (only StatusError should be included)
		statuses, err := th.HistoryStore.ListStatuses(th.Context,
			models.WithStatuses([]scheduler.Status{scheduler.StatusError}),
			models.WithFrom(models.NewUTC(ts)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 1)
		assert.Equal(t, "workflow-id-2", statuses[0].WorkflowID)
		assert.Equal(t, scheduler.StatusError, statuses[0].Status)
	})

	t.Run("LimitResults", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create multiple records
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := 1; i <= 5; i++ {
			th.CreateRun(t, ts, fmt.Sprintf("workflow-id-%d", i), scheduler.StatusSuccess)
		}

		// Limit to 3 results
		options := &models.ListStatusesOptions{Limit: 3}
		statuses, err := th.HistoryStore.ListStatuses(th.Context, func(o *models.ListStatusesOptions) {
			o.Limit = options.Limit
		}, models.WithFrom(models.NewUTC(ts)))

		require.NoError(t, err)
		require.Len(t, statuses, 3)
	})

	t.Run("SortByStartedAt", func(t *testing.T) {
		th := setupTestLocalStore(t)

		// Create records with different timestamps
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusSuccess)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusSuccess)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Get all statuses
		statuses, err := th.HistoryStore.ListStatuses(
			th.Context, models.WithFrom(models.NewUTC(ts1)),
		)

		require.NoError(t, err)
		require.Len(t, statuses, 3)

		// Verify they are sorted by StartedAt in descending order
		assert.Equal(t, "workflow-id-3", statuses[0].WorkflowID)
		assert.Equal(t, "workflow-id-2", statuses[1].WorkflowID)
		assert.Equal(t, "workflow-id-1", statuses[2].WorkflowID)
	})
}
