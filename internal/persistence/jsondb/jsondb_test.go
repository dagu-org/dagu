package jsondb

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJSONDB(t *testing.T) {
	t.Run("RecentRecords", func(t *testing.T) {
		th := setupTestJSONDB(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		// Request 2 most recent records
		records := th.DB.Recent(th.Context, "test_DAG", 2)
		require.Len(t, records, 2)

		status0, err := records[0].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-3", status0.RequestID)

		status1, err := records[1].ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status1.RequestID)

		// Request more than exist
		records = th.DB.Recent(th.Context, "test_DAG", 5)
		require.Len(t, records, 3)
	})
	t.Run("LatestRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		th.DB.latestStatusToday = false
		record, err := th.DB.Latest(th.Context, "test_DAG")
		require.NoError(t, err)

		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "request-id-3", status.RequestID)
	})
	t.Run("FindByRequestID", func(t *testing.T) {
		th := setupTestJSONDB(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		record, err := th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-2")
		require.NoError(t, err)

		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, "request-id-2", status.RequestID)

		// non-existent request ID
		_, err = th.DB.FindByRequestID(th.Context, "test_DAG", "nonexistent-id")
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})
	t.Run("UpdateRecord", func(t *testing.T) {
		th := setupTestJSONDB(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)

		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)

		record, err := th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-1")
		require.NoError(t, err)

		status, err := record.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, scheduler.StatusRunning.String(), status.Status.String())

		status.Status = scheduler.StatusSuccess
		err = th.DB.Update(th.Context, "test_DAG", "request-id-1", *status)
		require.NoError(t, err)

		// Verify the status is updated
		record, err = th.DB.FindByRequestID(th.Context, "test_DAG", "request-id-1")
		require.NoError(t, err)

		status, err = record.ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, scheduler.StatusSuccess.String(), status.Status.String())
	})
	t.Run("RemoveOld", func(t *testing.T) {
		th := setupTestJSONDB(t)

		ts1 := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
		ts2 := time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC)
		ts3 := time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC)

		th.CreateRecord(t, ts1, "request-id-1", scheduler.StatusRunning)
		th.CreateRecord(t, ts2, "request-id-2", scheduler.StatusError)
		th.CreateRecord(t, ts3, "request-id-3", scheduler.StatusSuccess)

		records := th.DB.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 3)

		err := th.DB.RemoveOld(th.Context, "test_DAG", 0)
		require.NoError(t, err)

		records = th.DB.Recent(th.Context, "test_DAG", 3)
		require.Len(t, records, 0)
	})
}
