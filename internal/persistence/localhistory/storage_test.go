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
		th := setupTestJSONDB(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		// Request 2 most recent records
		records := th.Repo.Recent(th.Context, "test_DAG", 2)
		require.Len(t, records, 2)

		// Verify the first record is the most recent
		status0, err := records[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-3", status0.ReqID)

		// Verify the second record is the second most recent
		status1, err := records[1].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status1.ReqID)

		// Verify all records are returned if the number requested is equal to the number of records
		records = th.Repo.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		records = th.Repo.Recent(th.Context, "test_DAG", 4)
		require.Len(t, records, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		// Set the database to return the latest status (even if it was created today)
		// Verify that record created before today is returned
		obj := th.Repo.(*historyStorage)
		obj.latestStatusToday = false
		record, err := th.Repo.Latest(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "request-id-3", status.ReqID)
	})
	t.Run("FindByReqID", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		// Find the record with request ID "request-id-2"
		record, err := th.Repo.Find(th.Context, "test_DAG", "request-id-2")
		require.NoError(t, err)

		// Verify the record is the correct one
		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status.ReqID)

		// Verify an error is returned if the request ID does not exist
		_, err = th.Repo.Find(th.Context, "test_DAG", "nonexistent-id")
		assert.ErrorIs(t, err, models.ErrReqIDNotFound)
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create timestamps for the records
		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		// Create records with different statuses
		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		// Verify records are present
		records := th.Repo.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 3)

		// Remove records older than 0 days
		// It should remove all records
		err := th.Repo.RemoveOld(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		// Verify records are removed
		records = th.Repo.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 0)
	})
	t.Run("SubRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRecord(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a sub record
		rootRun := digraph.NewRootRun("test_DAG", "parent-id")
		subDAG := th.DAG("sub_dag")
		record, err := th.Repo.Create(th.Context, subDAG.DAG, ts, "sub-id", models.NewRecordOptions{
			Root: &rootRun,
		})
		require.NoError(t, err)

		// Write the status
		err = record.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = record.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(subDAG.DAG)
		statusToWrite.ReqID = "sub-id"
		err = record.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify record is created
		existingRecord, err := th.Repo.FindSubRun(th.Context, rootRun.Name, rootRun.ReqID, "sub-id")
		require.NoError(t, err)

		status, err := existingRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.ReqID)
	})
	t.Run("SubRecord_Retry", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRecord(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a sub record
		rootRun := digraph.NewRootRun("test_DAG", "parent-id")
		subDAG := th.DAG("sub_dag")
		record, err := th.Repo.Create(th.Context, subDAG.DAG, ts, "sub-id", models.NewRecordOptions{
			Root: &rootRun,
		})
		require.NoError(t, err)

		// Write the status
		err = record.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = record.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(subDAG.DAG)
		statusToWrite.ReqID = "sub-id"
		statusToWrite.Status = scheduler.StatusRunning
		err = record.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Find the sub run by request ID
		ts = time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		existingRecord, err := th.Repo.FindSubRun(th.Context, rootRun.Name, rootRun.ReqID, "sub-id")
		require.NoError(t, err)
		existingRecordStatus, err := existingRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", existingRecordStatus.ReqID)
		assert.Equal(t, scheduler.StatusRunning.String(), existingRecordStatus.Status.String())

		// Create a retry record and write different status
		retryRecord, err := th.Repo.Create(th.Context, subDAG.DAG, ts, "sub-id", models.NewRecordOptions{
			Root:  &rootRun,
			Retry: true,
		})
		require.NoError(t, err)
		statusToWrite.Status = scheduler.StatusSuccess
		_ = retryRecord.Open(th.Context)
		_ = retryRecord.Write(th.Context, statusToWrite)
		_ = retryRecord.Close(th.Context)

		// Verify the retry record is created
		existingRecord, err = th.Repo.FindSubRun(th.Context, rootRun.Name, rootRun.ReqID, "sub-id")
		require.NoError(t, err)
		existingRecordStatus, err = existingRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", existingRecordStatus.ReqID)
		assert.Equal(t, scheduler.StatusSuccess.String(), existingRecordStatus.Status.String())
	})
	t.Run("ReadDAG", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		rec := th.CreateRecord(t, ts, "parent-id", scheduler.StatusRunning)

		// Write the status
		err := rec.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = rec.Close(th.Context)
		}()

		statusToWrite := models.InitialStatus(rec.dag)
		statusToWrite.ReqID = "parent-id"

		err = rec.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Read the DAG and verify it matches the original
		dag, err := rec.ReadDAG(th.Context)
		require.NoError(t, err)

		require.NotNil(t, dag)
		require.Equal(t, *rec.dag, *dag)
	})
}
