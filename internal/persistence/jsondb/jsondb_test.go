package jsondb

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/jsondb/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryData_Read(t *testing.T) {
	t.Parallel()

	t.Run("Recent", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_read_recent")

		for i := 0; i < 5; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := th.DB.NewRecord(th.Context, dag.Location, now, requestID)
			err := record.Open(th.Context)
			require.NoError(t, err)

			status := persistence.NewStatusFactory(dag.DAG).Create(requestID, scheduler.StatusRunning, 12345, time.Now())
			status.RequestID = requestID

			err = record.Write(th.Context, status)
			require.NoError(t, err)
			err = record.Close(th.Context)
			require.NoError(t, err)
		}

		statuses := th.DB.Recent(th.Context, dag.Location, 3)
		assert.Len(t, statuses, 3)

		first, err := statuses[0].ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "request-id-0", first.RequestID)
	})

	t.Run("LatestToday", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_read_today")
		requestID := "request-id-today"
		now := time.Now()

		record := th.DB.NewRecord(th.Context, dag.Location, now, requestID)
		err := record.Open(th.Context)
		require.NoError(t, err)

		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusRunning, 12345, time.Now(),
		)
		status.RequestID = requestID
		err = record.Write(th.Context, status)
		require.NoError(t, err)
		err = record.Close(th.Context)
		require.NoError(t, err)

		th.DB.latestStatusToday = true
		todaysRecord, err := th.DB.Latest(th.Context, dag.Location)
		require.NoError(t, err)

		todaysStatus, err := todaysRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, requestID, todaysStatus.RequestID)
	})

	t.Run("Latest", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_no_status_today")

		// Create status from yesterday
		yesterdayTime := time.Now().AddDate(0, 0, -1)
		requestID := "request-id-yesterday"

		record := th.DB.NewRecord(th.Context, dag.Location, yesterdayTime, requestID)

		err := record.Open(th.Context)
		require.NoError(t, err)

		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, 12345, time.Now(),
		)
		status.RequestID = requestID

		err = record.Write(th.Context, status)
		require.NoError(t, err)
		err = record.Close(th.Context)
		require.NoError(t, err)

		// Try to read today's status and expect an error
		_, err = th.DB.Latest(th.Context, dag.Location)
		assert.ErrorIs(t, err, persistence.ErrNoStatusData)

		// Read the latest status
		th.DB.latestStatusToday = false
		latestRecord, err := th.DB.Latest(th.Context, dag.Location)
		require.NoError(t, err)

		// Read the status from the latest record
		latestStatus, err := latestRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, requestID, latestStatus.RequestID)
	})

	t.Run("NoFilesExist", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_no_files")
		statuses := th.DB.Recent(th.Context, dag.Location, 5)
		assert.Empty(t, statuses)

		_, err := th.DB.Latest(th.Context, dag.Location)
		assert.ErrorIs(t, err, persistence.ErrNoStatusData)
	})

	t.Run("RequestedMoreThanExist", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_fewer_files")

		// Create 3 status entries
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := th.DB.NewRecord(th.Context, dag.Location, now, requestID)
			err := record.Open(th.Context)
			require.NoError(t, err)

			status := persistence.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, 12345, time.Now(),
			)

			err = record.Write(th.Context, status)
			require.NoError(t, err)
			err = record.Close(th.Context)
			require.NoError(t, err)
		}

		// Request more than exist
		statuses := th.DB.Recent(th.Context, dag.Location, 5)
		assert.Len(t, statuses, 3)
	})

	t.Run("FindByRequestIDNotFound", func(t *testing.T) {
		th := testSetup(t)
		dag := th.DAG("test_not_found")
		_, err := th.DB.FindByRequestID(th.Context, dag.Location, "nonexistent-id")
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})
}

func TestHistoryData_Update(t *testing.T) {
	th := testSetup(t)

	t.Run("UpdateNonExistentStatus", func(t *testing.T) {
		dag := th.DAG("test_update_nonexistent")
		requestID := "request-id-nonexistent"
		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, 12345, time.Now(),
		)
		err := th.DB.Update(th.Context, dag.Location, "nonexistent-id", status)
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})

	t.Run("UpdateWithEmptyRequestID", func(t *testing.T) {
		dag := th.DAG("test_update_empty_id")
		requestID := ""
		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, 12345, time.Now(),
		)
		err := th.DB.Update(th.Context, dag.Location, "", status)
		assert.ErrorIs(t, err, ErrRequestIDEmpty)
	})
}

func TestHistoryData_Remove(t *testing.T) {
	th := testSetup(t)

	t.Run("RemoveAllFiles", func(t *testing.T) {
		dag := th.DAG("test_remove_all")

		// Create multiple status files
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := th.DB.NewRecord(th.Context, dag.Location, now, requestID)

			err := record.Open(th.Context)
			require.NoError(t, err)

			status := persistence.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, 12345, time.Now(),
			)

			err = record.Write(th.Context, status)
			require.NoError(t, err)

			err = record.Close(th.Context)
			require.NoError(t, err)
		}

		// Verify files exist
		records := th.DB.Recent(th.Context, dag.Location, 5)
		assert.Len(t, records, 3)

		// Remove all files
		err := th.DB.RemoveAll(th.Context, dag.Location)
		require.NoError(t, err)

		// Verify all files are removed
		records = th.DB.Recent(th.Context, dag.Location, 5)
		assert.Empty(t, records)
	})

	t.Run("RemoveAllNonExistent", func(t *testing.T) {
		dag := th.DAG("test_remove_all_nonexistent")
		err := th.DB.RemoveAll(th.Context, dag.Location)
		assert.NoError(t, err)
	})

	t.Run("RemoveOld", func(t *testing.T) {
		dag := th.DAG("test_remove_old")

		// Create status file
		requestID := "request-id-old"
		oldTime := time.Now().AddDate(0, 0, -10)

		record := th.DB.NewRecord(th.Context, dag.Location, oldTime, requestID)
		err := record.Open(th.Context)
		require.NoError(t, err)

		status := persistence.NewStatusFactory(dag.DAG).Create(requestID, scheduler.StatusSuccess, 12345, time.Now())

		err = record.Write(th.Context, status)
		require.NoError(t, err)

		err = record.Close(th.Context)
		require.NoError(t, err)

		// Get the file path and update its modification time
		st := storage.New()
		addr := storage.NewAddress(th.tmpDir, dag.Name)
		filePath := st.GenerateFilePath(th.Context, addr, storage.NewUTC(oldTime), requestID)

		oldDate := time.Now().AddDate(0, 0, -10)
		err = os.Chtimes(filePath, oldDate, oldDate)
		require.NoError(t, err)

		// Remove files older than 5 days
		err = th.DB.RemoveOld(th.Context, dag.Location, 5)
		require.NoError(t, err)

		// Verify old file is removed
		_, err = th.DB.FindByRequestID(th.Context, dag.Location, requestID)
		assert.Error(t, err)
	})
}
