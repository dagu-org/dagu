package jsondb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHistoryData_Read(t *testing.T) {
	th := testSetup(t)

	t.Run("Recent", func(t *testing.T) {
		dag := th.DAG("test_read_recent")
		data := th.DB.Data(th.Context, dag.Location)

		for i := 0; i < 5; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := data.NewRecord(th.Context, now, requestID)
			err := record.Open(th.Context)
			require.NoError(t, err)

			status := persistence.NewStatusFactory(dag.DAG).Create(requestID, scheduler.StatusRunning, 12345, time.Now())
			status.RequestID = requestID

			err = record.Write(th.Context, status)
			require.NoError(t, err)
			err = record.Close(th.Context)
			require.NoError(t, err)
		}

		statuses := data.Recent(th.Context, 3)
		assert.Len(t, statuses, 3)

		first, err := statuses[0].ReadStatus(th.Context)
		require.NoError(t, err)

		assert.Equal(t, "request-id-0", first.RequestID)
	})

	t.Run("LatestToday", func(t *testing.T) {
		dag := th.DAG("test_read_today")
		requestID := "request-id-today"
		now := time.Now()
		data := th.DB.Data(th.Context, dag.Location)

		record := data.NewRecord(th.Context, now, requestID)
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

		todaysRecord, err := data.LatestToday(th.Context)
		require.NoError(t, err)

		todaysStatus, err := todaysRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, requestID, todaysStatus.RequestID)
	})

	t.Run("Latest", func(t *testing.T) {
		dag := th.DAG("test_no_status_today")
		data := th.DB.Data(th.Context, dag.Location)

		// Create status from yesterday
		yesterdayTime := time.Now().AddDate(0, 0, -1)
		requestID := "request-id-yesterday"

		record := data.NewRecord(th.Context, yesterdayTime, requestID)

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
		_, err = data.LatestToday(th.Context)
		assert.ErrorIs(t, err, persistence.ErrNoStatusData)

		// Read the latest status
		latestRecord, err := data.Latest(th.Context)
		require.NoError(t, err)

		// Read the status from the latest record
		latestStatus, err := latestRecord.ReadStatus(th.Context)
		require.NoError(t, err)
		assert.Equal(t, requestID, latestStatus.RequestID)
	})

	t.Run("NoFilesExist", func(t *testing.T) {
		dag := th.DAG("test_no_files")
		data := th.DB.Data(th.Context, dag.Location)
		statuses := data.Recent(th.Context, 5)
		assert.Empty(t, statuses)

		_, err := data.LatestToday(th.Context)
		assert.ErrorIs(t, err, persistence.ErrNoStatusData)

		_, err = data.Latest(th.Context)
		assert.ErrorIs(t, err, persistence.ErrNoStatusData)
	})

	t.Run("RequestedMoreThanExist", func(t *testing.T) {
		dag := th.DAG("test_fewer_files")
		data := th.DB.Data(th.Context, dag.Location)

		// Create 3 status entries
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := data.NewRecord(th.Context, now, requestID)
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
		statuses := data.Recent(th.Context, 5)
		assert.Len(t, statuses, 3)
	})

	t.Run("FindByRequestIDNotFound", func(t *testing.T) {
		dag := th.DAG("test_not_found")
		data := th.DB.Data(th.Context, dag.Location)
		_, err := data.FindByRequestID(th.Context, "nonexistent-id")
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		err := th.DB.Rename(th.Context, "relative/path", "/absolute/path")
		assert.Error(t, err)
	})
}

func TestHistoryData_Update(t *testing.T) {
	th := testSetup(t)

	t.Run("UpdateNonExistentStatus", func(t *testing.T) {
		dag := th.DAG("test_update_nonexistent")
		requestID := "request-id-nonexistent"
		data := th.DB.Data(th.Context, dag.Location)
		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, 12345, time.Now(),
		)
		err := data.Update(th.Context, "nonexistent-id", status)
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})

	t.Run("UpdateWithEmptyRequestID", func(t *testing.T) {
		dag := th.DAG("test_update_empty_id")
		requestID := ""
		status := persistence.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, 12345, time.Now(),
		)
		data := th.DB.Data(th.Context, dag.Location)
		err := data.Update(th.Context, "", status)
		assert.ErrorIs(t, err, ErrRequestIDEmpty)
	})
}

func TestHistoryData_Remove(t *testing.T) {
	th := testSetup(t)

	t.Run("RemoveAllFiles", func(t *testing.T) {
		dag := th.DAG("test_remove_all")
		data := th.DB.Data(th.Context, dag.Location)

		// Create multiple status files
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			record := data.NewRecord(th.Context, now, requestID)

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
		matches, err := filepath.Glob(data.globPattern(dag.Location))
		require.NoError(t, err)
		assert.Len(t, matches, 3)

		// Remove all files
		err = th.DB.RemoveAll(th.Context, dag.Location)
		require.NoError(t, err)

		// Verify all files are removed
		matches, err = filepath.Glob(data.globPattern(dag.Location))
		require.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("RemoveAllNonExistent", func(t *testing.T) {
		dag := th.DAG("test_remove_all_nonexistent")
		err := th.DB.RemoveAll(th.Context, dag.Location)
		assert.NoError(t, err)
	})

	t.Run("RemoveOld", func(t *testing.T) {
		dag := th.DAG("test_remove_old")
		data := th.DB.Data(th.Context, dag.Name)

		// Create status file
		requestID := "request-id-old"
		oldTime := time.Now().AddDate(0, 0, -10)

		record := data.NewRecord(th.Context, oldTime, requestID)
		err := record.Open(th.Context)
		require.NoError(t, err)

		status := persistence.NewStatusFactory(dag.DAG).Create(requestID, scheduler.StatusSuccess, 12345, time.Now())

		err = record.Write(th.Context, status)
		require.NoError(t, err)

		err = record.Close(th.Context)
		require.NoError(t, err)

		// Get the file path and update its modification time
		filePath := data.generateFilePath(th.Context, newUTC(oldTime), requestID)

		oldDate := time.Now().AddDate(0, 0, -10)
		err = os.Chtimes(filePath, oldDate, oldDate)
		require.NoError(t, err)

		// Remove files older than 5 days
		err = data.RemoveOld(th.Context, 5)
		require.NoError(t, err)

		// Verify old file is removed
		_, err = data.FindByRequestID(th.Context, requestID)
		assert.Error(t, err)
	})
}
