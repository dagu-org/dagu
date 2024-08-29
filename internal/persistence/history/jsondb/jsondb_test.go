// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package jsondb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testHelper struct {
	JSONDB *JSONDB
	TmpDir string
}

func setupTest(t *testing.T) testHelper {
	tmpDir, err := os.MkdirTemp("", "test-persistence")
	require.NoError(t, err)
	th := testHelper{
		JSONDB: New(tmpDir, logger.Default, true),
		TmpDir: tmpDir,
	}
	t.Cleanup(th.cleanup)
	return th
}

func (th testHelper) cleanup() {
	_ = os.RemoveAll(th.TmpDir)
}

func createTestDAG(th testHelper, name, location string) *dag.DAG {
	return &dag.DAG{
		Name:     name,
		Location: filepath.Join(th.TmpDir, location),
	}
}

func createTestStatus(d *dag.DAG, status scheduler.Status, pid int) *model.Status {
	return model.NewStatus(d, nil, status, pid, nil, nil)
}

func writeTestStatus(t *testing.T, db *JSONDB, d *dag.DAG, status *model.Status, tm time.Time) {
	require.NoError(t, db.Open(d.Location, tm, status.RequestID))
	require.NoError(t, db.Write(status))
	require.NoError(t, db.Close())
}

func TestNewJSONDB(t *testing.T) {
	th := setupTest(t)

	assert.NotNil(t, th.JSONDB)
	assert.Equal(t, th.TmpDir, th.JSONDB.baseDir)
	assert.True(t, th.JSONDB.latestStatusToday)
	assert.NotNil(t, th.JSONDB.cache)
}

func TestJSONDB_BasicOperations(t *testing.T) {
	th := setupTest(t)

	t.Run("GetLatest", func(t *testing.T) {
		d := createTestDAG(th, "test_get_latest", "test_get_latest.yaml")
		now := time.Now().UTC()

		// Create multiple statuses
		for i := 0; i < 3; i++ {
			status := createTestStatus(d, scheduler.StatusRunning, 10000+i)
			status.RequestID = fmt.Sprintf("request-id-%d", i+1)
			writeTestStatus(t, th.JSONDB, d, status, now.Add(time.Duration(i)*time.Hour))
		}

		// Get the latest status
		latestStatus, err := th.JSONDB.GetLatest(d.Location)
		require.NoError(t, err)
		assert.NotNil(t, latestStatus)
		assert.Equal(t, "request-id-3", latestStatus.RequestID)
	})

	t.Run("GetLatest_NoStatus", func(t *testing.T) {
		d := createTestDAG(th, "test_get_latest_no_status", "test_get_latest_no_status.yaml")

		// Try to get the latest status when no status exists
		_, err := th.JSONDB.GetLatest(d.Location)
		assert.Error(t, err)
	})

	t.Run("Open", func(t *testing.T) {
		d := createTestDAG(th, "test_open", "test_open.yaml")
		requestID := "request-id-1"
		now := time.Now()

		err := th.JSONDB.Open(d.Location, now, requestID)
		require.NoError(t, err)

		indexDir := filepath.Join(th.TmpDir, "index", d.Name)
		statusDir := filepath.Join(th.TmpDir, "status", now.Format("2006"), now.Format("01"), now.Format("02"))

		assert.DirExists(t, indexDir)
		assert.DirExists(t, statusDir)

		require.NoError(t, th.JSONDB.Close())
	})

	t.Run("WriteAndClose", func(t *testing.T) {
		d := createTestDAG(th, "test_write", "test_write.yaml")
		requestID := "req-1"
		now := time.Now()

		require.NoError(t, th.JSONDB.Open(d.Location, now, requestID))

		status := createTestStatus(d, scheduler.StatusRunning, 12345)
		require.NoError(t, th.JSONDB.Write(status))

		require.NoError(t, th.JSONDB.Close())

		statusFiles := th.JSONDB.ListRecent(d.Location, 1)
		require.Len(t, statusFiles, 1)
		assert.Equal(t, status.RequestID, statusFiles[0].Status.RequestID)
		assert.Equal(t, status.Status, statusFiles[0].Status.Status)
		assert.Equal(t, status.PID, statusFiles[0].Status.PID)
	})
}

func TestJSONDB_StatusOperations(t *testing.T) {
	th := setupTest(t)

	t.Run("ReadStatusRecent", func(t *testing.T) {
		d := createTestDAG(th, "test_read_status_recent", "test_read_status_recent.yaml")

		testData := []struct {
			Status    *model.Status
			ReqID     string
			Timestamp time.Time
		}{
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-1", time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-2", time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-3", time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local)},
		}

		for _, data := range testData {
			data.Status.RequestID = data.ReqID
			writeTestStatus(t, th.JSONDB, d, data.Status, data.Timestamp)
		}

		recentStatus := th.JSONDB.ListRecent(d.Location, 2)
		require.Len(t, recentStatus, 2)
		assert.Equal(t, "request-id-3", recentStatus[0].Status.RequestID)
		assert.Equal(t, "request-id-2", recentStatus[1].Status.RequestID)
	})

	t.Run("FindByRequestID", func(t *testing.T) {
		d := createTestDAG(th, "test_find_by_request_id", "test_find_by_request_id.yaml")

		testData := []struct {
			Status    *model.Status
			ReqID     string
			Timestamp time.Time
		}{
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-1", time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-2", time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-3", time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local)},
		}

		for _, data := range testData {
			data.Status.RequestID = data.ReqID
			writeTestStatus(t, th.JSONDB, d, data.Status, data.Timestamp)
		}

		t.Run("ExistingRequestID", func(t *testing.T) {
			status, err := th.JSONDB.GetByRequestID(d.Location, "request-id-2")
			require.NoError(t, err)
			require.Equal(t, "request-id-2", status.Status.RequestID)
		})

		t.Run("NonExistentRequestID", func(t *testing.T) {
			status, err := th.JSONDB.GetByRequestID(d.Location, "request-id-10000")
			require.Error(t, err)
			require.Nil(t, status)
		})
	})

	t.Run("Update", func(t *testing.T) {
		d := createTestDAG(th, "test_update", "test_update.yaml")
		requestID := "request-id-1"
		now := time.Now()

		initialStatus := createTestStatus(d, scheduler.StatusRunning, 12345)
		initialStatus.RequestID = requestID
		writeTestStatus(t, th.JSONDB, d, initialStatus, now)

		updatedStatus := createTestStatus(d, scheduler.StatusSuccess, 12345)
		updatedStatus.RequestID = requestID
		require.NoError(t, th.JSONDB.UpdateStatus(d.Location, requestID, updatedStatus))

		statusFiles := th.JSONDB.ListRecent(d.Location, 1)
		require.Len(t, statusFiles, 1)
		assert.Equal(t, updatedStatus.RequestID, statusFiles[0].Status.RequestID)
		assert.Equal(t, updatedStatus.Status, statusFiles[0].Status.Status)
		assert.Equal(t, updatedStatus.PID, statusFiles[0].Status.PID)
	})
}

func TestJSONDB_FileOperations(t *testing.T) {
	th := setupTest(t)

	t.Run("RemoveOld", func(t *testing.T) {
		d := createTestDAG(th, "test_remove_old", "test_remove_old.yaml")

		testData := []struct {
			Status    *model.Status
			ReqID     string
			Timestamp time.Time
		}{
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-1", time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-2", time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-3", time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local)},
		}

		for _, data := range testData {
			data.Status.RequestID = data.ReqID
			writeTestStatus(t, th.JSONDB, d, data.Status, data.Timestamp)
		}

		files := th.JSONDB.ListRecent(d.Location, 3)
		require.Equal(t, 3, len(files))

		require.NoError(t, os.Chtimes(files[0].File, time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, -2)))
		require.NoError(t, os.Chtimes(files[1].File, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, -1)))

		require.NoError(t, th.JSONDB.DeleteOld(d.Location, 1))

		files = th.JSONDB.ListRecent(d.Location, 3)
		require.Equal(t, 1, len(files))
	})

	t.Run("RemoveAll", func(t *testing.T) {
		d := createTestDAG(th, "test_remove_all", "test_remove_all.yaml")

		testData := []struct {
			Status    *model.Status
			ReqID     string
			Timestamp time.Time
		}{
			{createTestStatus(d, scheduler.StatusNone, 10000), "req-1", time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "req-2", time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
			{createTestStatus(d, scheduler.StatusNone, 10000), "req-3", time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local)},
		}

		for _, data := range testData {
			data.Status.RequestID = data.ReqID
			writeTestStatus(t, th.JSONDB, d, data.Status, data.Timestamp)
		}

		files := th.JSONDB.ListRecent(d.Location, 3)
		require.Equal(t, 3, len(files))

		require.NoError(t, th.JSONDB.DeleteAll(d.Location))

		files = th.JSONDB.ListRecent(d.Location, 3)
		require.Empty(t, files)
	})

	t.Run("Rename", func(t *testing.T) {
		oldID := "old_dag"
		newID := "new_dag"
		d := createTestDAG(th, oldID, oldID+".yaml")

		status := createTestStatus(d, scheduler.StatusSuccess, 12345)
		status.RequestID = "request-id-1"
		writeTestStatus(t, th.JSONDB, d, status, time.Now())

		oldPath := d.Location
		newPath := filepath.Join(filepath.Dir(d.Location), newID+".yaml")

		require.NoError(t, th.JSONDB.RenameDAG(oldPath, newPath))

		oldIndexDir := filepath.Join(th.TmpDir, "index", oldID)
		assert.NoDirExists(t, oldIndexDir)

		newIndexDir := filepath.Join(th.TmpDir, "index", newID)
		assert.DirExists(t, newIndexDir)

		statusFiles := th.JSONDB.ListRecent(newPath, 1)
		require.Len(t, statusFiles, 1)
		assert.Equal(t, status.RequestID, statusFiles[0].Status.RequestID)
		assert.Equal(t, status.Status, statusFiles[0].Status.Status)
	})

	t.Run("Rename_Conflict", func(t *testing.T) {
		oldID := "old_dag"
		newID := "new_dag"
		d1 := createTestDAG(th, oldID, oldID+".yaml")
		d2 := createTestDAG(th, newID, newID+".yaml")

		status1 := createTestStatus(d1, scheduler.StatusSuccess, 12345)
		status1.RequestID = "request-id-1"
		writeTestStatus(t, th.JSONDB, d1, status1, time.Now())

		status2 := createTestStatus(d2, scheduler.StatusSuccess, 12346)
		status2.RequestID = "request-id-2"
		writeTestStatus(t, th.JSONDB, d2, status2, time.Now())

		err := th.JSONDB.RenameDAG(d1.Location, d2.Location)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflict")
	})
}

func TestJSONDB_ListingOperations(t *testing.T) {
	th := setupTest(t)

	t.Run("listRecentFiles", func(t *testing.T) {
		d := createTestDAG(th, "test_list_recent", "test_list_recent.yaml")

		for i := 0; i < 5; i++ {
			status := createTestStatus(d, scheduler.StatusNone, 10000+i)
			status.RequestID = fmt.Sprintf("request-id-%d", i+1)
			writeTestStatus(t, th.JSONDB, d, status, time.Now().AddDate(0, 0, -i))
		}

		files, err := th.JSONDB.listRecentFiles(filepath.Join(th.TmpDir, "status"), 3)
		require.NoError(t, err)
		assert.Len(t, files, 3)

		for i := 0; i < len(files)-1; i++ {
			assert.True(t, files[i] > files[i+1], "Files should be in reverse chronological order")
		}
	})

	t.Run("listDirsSorted", func(t *testing.T) {
		testDir := filepath.Join(th.TmpDir, "test_list_dirs")
		require.NoError(t, os.MkdirAll(testDir, 0755))

		testDirs := []string{"2023", "2022", "2021"}
		for _, dir := range testDirs {
			require.NoError(t, os.Mkdir(filepath.Join(testDir, dir), 0755))
		}

		t.Run("Ascending", func(t *testing.T) {
			dirs, err := listDirsSorted(testDir, false)
			require.NoError(t, err)
			assert.Equal(t, []string{"2021", "2022", "2023"}, dirs)
		})

		t.Run("Descending", func(t *testing.T) {
			dirs, err := listDirsSorted(testDir, true)
			require.NoError(t, err)
			assert.Equal(t, []string{"2023", "2022", "2021"}, dirs)
		})
	})

	t.Run("listFilesSorted", func(t *testing.T) {
		testDir := filepath.Join(th.TmpDir, "test_list_files")
		require.NoError(t, os.MkdirAll(testDir, 0755))

		testFiles := []string{"file1.dat", "file2.dat", "file3.dat"}
		for _, file := range testFiles {
			_, err := os.Create(filepath.Join(testDir, file))
			require.NoError(t, err)
		}

		t.Run("Ascending", func(t *testing.T) {
			files, err := listFilesSorted(testDir, false)
			require.NoError(t, err)
			assert.Len(t, files, 3)
			for i, file := range files {
				assert.Equal(t, filepath.Join(testDir, testFiles[i]), file)
			}
		})

		t.Run("Descending", func(t *testing.T) {
			files, err := listFilesSorted(testDir, true)
			require.NoError(t, err)
			assert.Len(t, files, 3)
			for i, file := range files {
				assert.Equal(t, filepath.Join(testDir, testFiles[len(testFiles)-1-i]), file)
			}
		})
	})
}

func TestJSONDB_SpecialOperations(t *testing.T) {
	th := setupTest(t)

	t.Run("latestToday", func(t *testing.T) {
		d := createTestDAG(th, "test_latest_today", "test_latest_today.yaml")
		now := time.Date(2022, 1, 1, 0, 0, 0, 0, time.UTC)

		for i := 0; i < 3; i++ {
			status := createTestStatus(d, scheduler.StatusNone, 10000+i)
			status.RequestID = fmt.Sprintf("req-%d", i+1)
			writeTestStatus(t, th.JSONDB, d, status, now.Add(time.Duration(i)*time.Hour))
		}

		latestFile, err := th.JSONDB.latestToday(d.Location, now, true)
		require.NoError(t, err)
		assert.Contains(t, latestFile, now.Format("20060102"))
		assert.Contains(t, latestFile, "req-3")
	})

	t.Run("craftCompactedFileName", func(t *testing.T) {
		originalFile := "/path/to/status_file.dat"
		compactedFile, err := craftCompactedFileName(originalFile)
		require.NoError(t, err)
		assert.Equal(t, "/path/to/status_file_c.dat", compactedFile)

		_, err = craftCompactedFileName("/path/to/status_file_c.dat")
		require.Error(t, err)
	})
}

func TestJSONDB_ListStatusesByDate(t *testing.T) {
	th := setupTest(t)

	loc, err := time.LoadLocation("Local")
	require.NoError(t, err)
	date := time.Date(2023, 5, 15, 0, 0, 0, 0, loc)

	dags := []string{"dag1", "dag2", "dag3"}
	totalEntries := 0

	for _, dagID := range dags {
		for i := 0; i < 3; i++ {
			requestID := fmt.Sprintf("%s-req-%d", dagID, i)
			timestamp := date.Add(time.Duration(i) * time.Hour)

			err := th.JSONDB.Open(dagID, timestamp, requestID)
			require.NoError(t, err)

			status := &model.Status{
				Name:      dagID,
				RequestID: requestID,
				StartedAt: timestamp.Format(time.RFC3339),
				Status:    scheduler.StatusRunning,
			}
			err = th.JSONDB.Write(status)
			require.NoError(t, err)

			err = th.JSONDB.Close()
			require.NoError(t, err)

			totalEntries++
		}
	}

	statusFiles, err := th.JSONDB.ListByLocalDate(date)
	require.NoError(t, err)
	assert.Len(t, statusFiles, totalEntries)

	for i := 0; i < len(statusFiles)-1; i++ {
		t1, _ := time.Parse(time.RFC3339, statusFiles[i].Status.StartedAt)
		t2, _ := time.Parse(time.RFC3339, statusFiles[i+1].Status.StartedAt)
		assert.True(t, t1.Equal(t2) || t1.After(t2))
	}

	dagSet := make(map[string]bool)
	for _, status := range statusFiles {
		dagSet[status.Status.Name] = true
	}
	assert.Equal(t, len(dags), len(dagSet), "Statuses should be from all DAGs")

	emptyDate := date.AddDate(0, 0, 1)
	emptyStatusFiles, err := th.JSONDB.ListByLocalDate(emptyDate)
	require.NoError(t, err)
	assert.Empty(t, emptyStatusFiles)

	edgeDate := time.Date(2023, 3, 26, 1, 30, 0, 0, loc)
	err = th.JSONDB.Open("edge-dag", edgeDate, "edge-request")
	require.NoError(t, err)
	edgeStatus := &model.Status{
		Name:      "edge-dag",
		RequestID: "edge-request",
		StartedAt: edgeDate.Format(time.RFC3339),
		Status:    scheduler.StatusRunning,
	}
	err = th.JSONDB.Write(edgeStatus)
	require.NoError(t, err)
	err = th.JSONDB.Close()
	require.NoError(t, err)

	edgeStatusFiles, err := th.JSONDB.ListByLocalDate(edgeDate)
	require.NoError(t, err)
	assert.Len(t, edgeStatusFiles, 1)
	assert.Equal(t, "edge-dag", edgeStatusFiles[0].Status.Name)
}

func TestJSONDB_ListRecentStatusAllDAGs(t *testing.T) {
	th := setupTest(t)

	dags := []struct {
		name     string
		location string
	}{
		{"dag1", "dag1.yaml"},
		{"dag2", "dag2.yaml"},
		{"dag3", "dag3.yaml"},
	}

	now := time.Now()

	for i, d := range dags {
		dag := createTestDAG(th, d.name, d.location)
		for j := 0; j < 3; j++ {
			tm := now.Add(-time.Duration(i*3+j) * time.Hour)
			status := createTestStatus(dag, scheduler.Status(j), 10000+i*3+j)
			status.RequestID = fmt.Sprintf("req-%d-%d", i, j)
			status.StartedAt = tm.Format(time.RFC3339)
			writeTestStatus(t, th.JSONDB, dag, status, tm)
		}
	}

	recentStatus, err := th.JSONDB.ListRecentAll(5)
	require.NoError(t, err)
	assert.Len(t, recentStatus, 5)

	for i := 0; i < len(recentStatus)-1; i++ {
		assert.True(t, recentStatus[i].Status.StartedAt > recentStatus[i+1].Status.StartedAt)
	}

	allStatus, err := th.JSONDB.ListRecentAll(100)
	require.NoError(t, err)
	assert.Len(t, allStatus, 9)

	zeroStatus, err := th.JSONDB.ListRecentAll(0)
	require.NoError(t, err)
	assert.Empty(t, zeroStatus)

	th.JSONDB.baseDir = "/non/existent/dir"
	_, err = th.JSONDB.ListRecentAll(5)
	assert.Error(t, err)
}

func TestJSONDB_EdgeCases(t *testing.T) {
	th := setupTest(t)

	t.Run("WriteWithoutOpen", func(t *testing.T) {
		d := createTestDAG(th, "test_write_without_open", "test_write_without_open.yaml")
		status := createTestStatus(d, scheduler.StatusRunning, 12345)
		err := th.JSONDB.Write(status)
		assert.Error(t, err)
	})

	t.Run("CloseWithoutOpen", func(t *testing.T) {
		err := th.JSONDB.Close()
		assert.Error(t, err)
	})

	t.Run("DeleteOldWithInvalidDuration", func(t *testing.T) {
		d := createTestDAG(th, "test_delete_old_invalid", "test_delete_old_invalid.yaml")
		err := th.JSONDB.DeleteOld(d.Location, -1)
		assert.Error(t, err)
	})

	t.Run("RenameNonExistentDAG", func(t *testing.T) {
		oldPath := filepath.Join(th.TmpDir, "non_existent_old.yaml")
		newPath := filepath.Join(th.TmpDir, "non_existent_new.yaml")
		err := th.JSONDB.RenameDAG(oldPath, newPath)
		// No error should be returned if the DAG does not exist
		assert.NoError(t, err)
	})
}
