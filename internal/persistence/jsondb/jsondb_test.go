package jsondb

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
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
		records := th.DB.Recent(th.Context, "test_DAG", 2)
		require.Len(t, records, 2)

		// Verify the first record is the most recent
		status0, err := records[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-3", status0.RequestID)

		// Verify the second record is the second most recent
		status1, err := records[1].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status1.RequestID)

		// Verify all records are returned if the number requested is equal to the number of records
		records = th.DB.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 3)

		// Verify all records are returned if the number requested is greater than the number of records
		records = th.DB.Recent(th.Context, "test_DAG", 4)
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
		th.DB.latestStatusToday = false
		record, err := th.DB.Latest(th.Context, "test_DAG")
		require.NoError(t, err)

		// Verify the record is the most recent
		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "request-id-3", status.RequestID)
	})
	t.Run("FindByRequestID", func(t *testing.T) {
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
		record, err := th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-2")
		require.NoError(t, err)

		// Verify the record is the correct one
		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status.RequestID)

		// Verify an error is returned if the request ID does not exist
		_, err = th.DB.FindByRequestID(th.Context, "test_DAG", "nonexistent-id")
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})
	t.Run("UpdateRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create a timestamp for the record
		timestamp := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		th.CreateRecord(t, timestamp, "request-id-1", scheduler.StatusRunning)

		// Verify the status is created
		record, err := th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-1")
		require.NoError(t, err)

		// Update the status
		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, scheduler.StatusRunning.String(), status.Status.String())

		// Update the status to success
		status.Status = scheduler.StatusSuccess
		err = th.DB.Update(th.Context, "test_DAG", "request-id-1", *status)
		require.NoError(t, err)

		// Verify the status is updated
		record, err = th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-1")
		require.NoError(t, err)

		// Verify the status is updated
		status, err = record.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
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
		records := th.DB.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 3)

		// Remove records older than 0 days
		// It should remove all records
		err := th.DB.RemoveOld(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		// Verify records are removed
		records = th.DB.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 0)
	})
	t.Run("SubRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		// Create a timestamp for the parent record
		ts := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		// Create a parent record
		_ = th.CreateRecord(t, ts, "parent-id", scheduler.StatusRunning)

		// Create a sub record
		rootDAG := digraph.NewRootDAG("test_DAG", "parent-id")
		subDAG := th.DAG("sub_dag")
		record, err := th.DB.NewSubRecord(th.Context, subDAG.DAG, ts, "sub-id", rootDAG)
		require.NoError(t, err)

		// Write the status
		err = record.Open(th.Context)
		require.NoError(t, err)
		defer record.Close(th.Context)

		statusToWrite := persistence.NewStatusFactory(subDAG.DAG).Default()
		statusToWrite.RequestID = "sub-id"
		err = record.Write(th.Context, statusToWrite)
		require.NoError(t, err)

		// Verify record is created
		existingRecord, err := th.DB.FindBySubRequestID(th.Context, "sub-id", rootDAG)
		require.NoError(t, err)

		status, err := existingRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "sub-id", status.RequestID)
	})
}
