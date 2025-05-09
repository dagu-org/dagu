package localhistory

import (
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
		th := setupTestLocalStorage(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Request 2 most recent runs
		runs := th.HistoryRepo.RecentRuns(th.Context, "test_DAG", 2)
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
		runs = th.HistoryRepo.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		runs = th.HistoryRepo.RecentRuns(th.Context, "test_DAG", 4)
		require.Len(t, runs, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestLocalStorage(t)

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
		obj := th.HistoryRepo.(*localStorage)
		obj.latestStatusToday = false
		run, err := th.HistoryRepo.LatestRun(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		status, err := run.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "workflow-id-3", status.WorkflowID)
	})
	t.Run("FindByWorkflowID", func(t *testing.T) {
		th := setupTestLocalStorage(t)

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
		run, err := th.HistoryRepo.FindRun(th.Context, ref)
		require.NoError(t, err)

		// Verify the record is the correct one
		status, err := run.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "workflow-id-2", status.WorkflowID)

		// Verify an error is returned if the workflow ID does not exist
		refNonExist := digraph.NewWorkflowRef("test_DAG", "nonexistent-id")
		_, err = th.HistoryRepo.FindRun(th.Context, refNonExist)
		assert.ErrorIs(t, err, models.ErrWorkflowIDNotFound)
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestLocalStorage(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRun(t, ts1, "workflow-id-1", scheduler.StatusRunning)
		th.CreateRun(t, ts2, "workflow-id-2", scheduler.StatusError)
		th.CreateRun(t, ts3, "workflow-id-3", scheduler.StatusSuccess)

		// Verify runs are present
		runs := th.HistoryRepo.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 3)

		// Remove records older than 0 days
		// It should remove all records
		err := th.HistoryRepo.RemoveOldWorkflows(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		// Verify records are removed
		runs = th.HistoryRepo.RecentRuns(th.Context, "test_DAG", 3)
		require.Len(t, runs, 0)
	})
	t.Run("ChildWorkflow", func(t *testing.T) {
		th := setupTestLocalStorage(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRun(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child run
		root := digraph.NewWorkflowRef("test_DAG", "parent-id")
		childWorkflowDAG := th.DAG("child")
		childRun, err := th.HistoryRepo.CreateRun(th.Context, childWorkflowDAG.DAG, ts, "sub-id", models.NewRunOptions{
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
		existingRun, err := th.HistoryRepo.FindChildWorkflowRun(th.Context, workflowRef, "sub-id")
		require.NoError(t, err)

		status, err := existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.WorkflowID)
	})
	t.Run("ChildWorkflow_Retry", func(t *testing.T) {
		th := setupTestLocalStorage(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRun(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a child workflow
		const childWorkflowID = "child-workflow-id"
		const parentExecID = "parent-id"

		root := digraph.NewWorkflowRef("test_DAG", parentExecID)
		childWorkflowDAG := th.DAG("child")
		run, err := th.HistoryRepo.CreateRun(th.Context, childWorkflowDAG.DAG, ts, childWorkflowID, models.NewRunOptions{
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
		existingRun, err := th.HistoryRepo.FindChildWorkflowRun(th.Context, workflowRef, childWorkflowID)
		require.NoError(t, err)
		existingRunStatus, err := existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childWorkflowID, existingRunStatus.WorkflowID)
		assert.Equal(t, scheduler.StatusRunning.String(), existingRunStatus.Status.String())

		// Create a retry record and write different status
		retryRun, err := th.HistoryRepo.CreateRun(th.Context, childWorkflowDAG.DAG, ts, childWorkflowID, models.NewRunOptions{
			Root:  &root,
			Retry: true,
		})
		require.NoError(t, err)
		statusToWrite.Status = scheduler.StatusSuccess
		_ = retryRun.Open(th.Context)
		_ = retryRun.Write(th.Context, statusToWrite)
		_ = retryRun.Close(th.Context)

		// Verify the retry record is created
		existingRun, err = th.HistoryRepo.FindChildWorkflowRun(th.Context, workflowRef, childWorkflowID)
		require.NoError(t, err)
		existingRunStatus, err = existingRun.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, childWorkflowID, existingRunStatus.WorkflowID)
		assert.Equal(t, scheduler.StatusSuccess.String(), existingRunStatus.Status.String())
	})
	t.Run("ReadDAG", func(t *testing.T) {
		th := setupTestLocalStorage(t)

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
