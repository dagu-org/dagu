package jsondb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testPID = 12345

func TestJSONDB_Basic(t *testing.T) {
	th := testSetup(t)

	t.Run("OpenAndClose", func(t *testing.T) {
		dag := th.DAG("test_open_close")
		requestID := "request-id-test-open-close"
		now := time.Now()

		err := th.DB.Open(th.Context, dag.Location, now, requestID)
		require.NoError(t, err)

		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusRunning, testPID, time.Now(),
		)
		err = th.DB.Write(th.Context, status)
		require.NoError(t, err)

		err = th.DB.Close(th.Context)
		require.NoError(t, err)
	})

	t.Run("UpdateStatus", func(t *testing.T) {
		dag := th.DAG("test_update")
		requestID := "request-id-test-update"
		now := time.Now()

		// Create initial status
		err := th.DB.Open(th.Context, dag.Location, now, requestID)
		require.NoError(t, err)

		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusRunning, testPID, time.Now(),
		)
		err = th.DB.Write(th.Context, status)
		require.NoError(t, err)
		err = th.DB.Close(th.Context)
		require.NoError(t, err)

		// Update status
		status.Status = scheduler.StatusSuccess
		err = th.DB.Update(th.Context, dag.Location, requestID, status)
		require.NoError(t, err)

		// Verify updated status
		statusFile, err := th.DB.FindByRequestID(th.Context, dag.Location, requestID)
		require.NoError(t, err)
		assert.Equal(t, scheduler.StatusSuccess, statusFile.Status.Status)
	})
}

func TestJSONDB_ReadStatus(t *testing.T) {
	th := testSetup(t)

	t.Run("ReadStatusRecent", func(t *testing.T) {
		dag := th.DAG("test_read_recent")

		// Create multiple status entries
		for i := 0; i < 5; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			err := th.DB.Open(th.Context, dag.Location, now, requestID)
			require.NoError(t, err)

			status := model.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, testPID, time.Now(),
			)
			status.RequestID = requestID
			err = th.DB.Write(th.Context, status)
			require.NoError(t, err)
			err = th.DB.Close(th.Context)
			require.NoError(t, err)
		}

		// Read recent status entries
		statuses := th.DB.ReadStatusRecent(th.Context, dag.Location, 3)
		assert.Len(t, statuses, 3)
		assert.Equal(t, "request-id-0", statuses[0].Status.RequestID)
	})

	t.Run("ReadStatusToday", func(t *testing.T) {
		dag := th.DAG("test_read_today")
		requestID := "request-id-today"
		now := time.Now()

		err := th.DB.Open(th.Context, dag.Location, now, requestID)
		require.NoError(t, err)

		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusRunning, testPID, time.Now(),
		)
		status.RequestID = requestID
		err = th.DB.Write(th.Context, status)
		require.NoError(t, err)
		err = th.DB.Close(th.Context)
		require.NoError(t, err)

		// Read today's status
		todayStatus, err := th.DB.ReadStatusToday(th.Context, dag.Location)
		require.NoError(t, err)
		assert.Equal(t, requestID, todayStatus.RequestID)
	})
}

func TestJSONDB_ReadStatusRecent_EdgeCases(t *testing.T) {
	th := testSetup(t)

	t.Run("NoFilesExist", func(t *testing.T) {
		dag := th.DAG("test_no_files")
		statuses := th.DB.ReadStatusRecent(th.Context, dag.Location, 5)
		assert.Empty(t, statuses)
	})

	t.Run("RequestedMoreThanExist", func(t *testing.T) {
		dag := th.DAG("test_fewer_files")

		// Create 3 status entries
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			err := th.DB.Open(th.Context, dag.Location, now, requestID)
			require.NoError(t, err)
			status := model.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, testPID, time.Now(),
			)
			err = th.DB.Write(th.Context, status)
			require.NoError(t, err)
			err = th.DB.Close(th.Context)
			require.NoError(t, err)
		}

		// Request more than exist
		statuses := th.DB.ReadStatusRecent(th.Context, dag.Location, 5)
		assert.Len(t, statuses, 3)
	})
}

func TestJSONDB_ReadStatusToday_EdgeCases(t *testing.T) {
	th := testSetup(t)

	t.Run("NoStatusToday", func(t *testing.T) {
		dag := th.DAG("test_no_status_today")

		// Create status from yesterday
		yesterdayTime := time.Now().AddDate(0, 0, -1)
		requestID := "request-id-yesterday"

		err := th.DB.Open(th.Context, dag.Location, yesterdayTime, requestID)
		require.NoError(t, err)
		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, testPID, time.Now(),
		)
		status.RequestID = requestID
		err = th.DB.Write(th.Context, status)
		require.NoError(t, err)
		err = th.DB.Close(th.Context)
		require.NoError(t, err)

		// Try to read today's status
		_, err = th.DB.ReadStatusToday(th.Context, dag.Location)
		assert.ErrorIs(t, err, persistence.ErrNoStatusDataToday)
	})

	t.Run("NoStatusData", func(t *testing.T) {
		dag := th.DAG("test_no_status_data")
		_, err := th.DB.ReadStatusToday(th.Context, dag.Location)
		assert.ErrorIs(t, err, persistence.ErrNoStatusDataToday)
	})
}

func TestJSONDB_RemoveAll(t *testing.T) {
	th := testSetup(t)

	t.Run("RemoveAllFiles", func(t *testing.T) {
		dag := th.DAG("test_remove_all")

		// Create multiple status files
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("request-id-%d", i)
			now := time.Now().Add(time.Duration(-i) * time.Hour)

			err := th.DB.Open(th.Context, dag.Location, now, requestID)
			require.NoError(t, err)
			status := model.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, testPID, time.Now(),
			)
			err = th.DB.Write(th.Context, status)
			require.NoError(t, err)
			err = th.DB.Close(th.Context)
			require.NoError(t, err)
		}

		// Verify files exist
		matches, err := filepath.Glob(th.DB.globPattern(dag.Location))
		require.NoError(t, err)
		assert.Len(t, matches, 3)

		// Remove all files
		err = th.DB.RemoveAll(th.Context, dag.Location)
		require.NoError(t, err)

		// Verify all files are removed
		matches, err = filepath.Glob(th.DB.globPattern(dag.Location))
		require.NoError(t, err)
		assert.Empty(t, matches)
	})

	t.Run("RemoveAllNonExistent", func(t *testing.T) {
		dag := th.DAG("test_remove_all_nonexistent")
		err := th.DB.RemoveAll(th.Context, dag.Location)
		assert.NoError(t, err)
	})
}

func TestJSONDB_Update_EdgeCases(t *testing.T) {
	th := testSetup(t)

	t.Run("UpdateNonExistentStatus", func(t *testing.T) {
		dag := th.DAG("test_update_nonexistent")
		requestID := "request-id-nonexistent"
		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, testPID, time.Now(),
		)
		err := th.DB.Update(th.Context, dag.Location, "nonexistent-id", status)
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})

	t.Run("UpdateWithEmptyRequestID", func(t *testing.T) {
		dag := th.DAG("test_update_empty_id")
		requestID := ""
		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, testPID, time.Now(),
		)
		err := th.DB.Update(th.Context, dag.Location, "", status)
		assert.ErrorIs(t, err, errRequestIDNotFound)
	})
}

func TestJSONDB_ErrorHandling(t *testing.T) {
	th := testSetup(t)

	t.Run("FindByRequestIDNotFound", func(t *testing.T) {
		dag := th.DAG("test_not_found")
		_, err := th.DB.FindByRequestID(th.Context, dag.Location, "nonexistent-id")
		assert.ErrorIs(t, err, persistence.ErrRequestIDNotFound)
	})

	t.Run("EmptyDAGFile", func(t *testing.T) {
		_, err := th.DB.generateFilePath("", newUTC(time.Now()), "request-id")
		assert.ErrorIs(t, err, errKeyEmpty)
	})

	t.Run("InvalidPath", func(t *testing.T) {
		err := th.DB.Rename(th.Context, "relative/path", "/absolute/path")
		assert.Error(t, err)
	})
}

func TestJSONDB_FileManagement(t *testing.T) {
	th := testSetup(t)

	t.Run("RemoveOld", func(t *testing.T) {
		dag := th.DAG("test_remove_old")

		// Create status file
		requestID := "request-id-old"
		oldTime := time.Now().AddDate(0, 0, -10)

		filePathOld, _ := th.DB.generateFilePath(dag.Location, newUTC(oldTime), requestID)
		println(filePathOld)
		err := th.DB.Open(th.Context, dag.Location, oldTime, requestID)
		require.NoError(t, err)

		status := model.NewStatusFactory(dag.DAG).Create(
			requestID, scheduler.StatusSuccess, testPID, time.Now(),
		)

		err = th.DB.Write(th.Context, status)
		require.NoError(t, err)
		err = th.DB.Close(th.Context)
		require.NoError(t, err)

		// Get the file path and update its modification time
		filePath, err := th.DB.generateFilePath(dag.Location, newUTC(oldTime), requestID)
		require.NoError(t, err)
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

	t.Run("Compact", func(t *testing.T) {
		dag := th.DAG("test_compact")
		requestID := "request-id-compact"
		now := time.Now()

		// Create a status file with multiple updates
		err := th.DB.Open(th.Context, dag.Location, now, requestID)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			status := model.NewStatusFactory(dag.DAG).Create(
				requestID, scheduler.StatusRunning, testPID, time.Now(),
			)
			err = th.DB.Write(th.Context, status)
			require.NoError(t, err)
		}

		filePath, err := th.DB.generateFilePath(dag.Location, newUTC(now), requestID)
		require.NoError(t, err)

		// Get file size before compaction
		info, err := os.Stat(filePath)
		require.NoError(t, err)
		sizeBeforeCompact := info.Size()

		// Compact the file
		err = th.DB.Close(th.Context) // Close will trigger compaction
		require.NoError(t, err)

		// Verify compacted file
		matches, err := filepath.Glob(th.DB.globPattern(dag.Location))
		require.NoError(t, err)
		require.Len(t, matches, 1)

		info, err = os.Stat(matches[0])
		require.NoError(t, err)
		assert.Less(t, info.Size(), sizeBeforeCompact)
	})
}
