// Copyright (C) 2024 The Daguflow/Dagu Authors
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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/dag/scheduler"
	"github.com/daguflow/dagu/internal/logger"
	"github.com/daguflow/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testEnv struct {
	JSONDB *JSONDB
	TmpDir string
}

func setup(t *testing.T) testEnv {
	tmpDir, err := os.MkdirTemp("", "test-persistence")
	require.NoError(t, err)
	return testEnv{
		JSONDB: New(tmpDir, logger.Default, true),
		TmpDir: tmpDir,
	}
}

func (te testEnv) cleanup() {
	_ = os.RemoveAll(te.TmpDir)
}

func createTestDAG(te testEnv, name, location string) *dag.DAG {
	return &dag.DAG{
		Name:     name,
		Location: filepath.Join(te.TmpDir, location),
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
	tmpDir, err := os.MkdirTemp("", "test-jsondb")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := logger.Default
	db := New(tmpDir, logger, true)

	assert.NotNil(t, db)
	assert.Equal(t, tmpDir, db.baseDir)
	assert.True(t, db.latestStatusToday)
	assert.NotNil(t, db.cache)
}

func TestJSONDB_Open(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_open", "test_open.yaml")
	requestID := "request-id-1"
	now := time.Now()

	err := te.JSONDB.Open(d.Location, now, requestID)
	require.NoError(t, err)

	// Verify that index and status files were created
	indexDir := filepath.Join(te.TmpDir, "index", d.Name)
	statusDir := filepath.Join(te.TmpDir, "status", now.Format("2006"), now.Format("01"), now.Format("02"))

	assert.DirExists(t, indexDir)
	assert.DirExists(t, statusDir)

	// Clean up
	require.NoError(t, te.JSONDB.Close())
}

func TestJSONDB_WriteAndClose(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_write", "test_write.yaml")
	requestID := "req-1"
	now := time.Now()

	require.NoError(t, te.JSONDB.Open(d.Location, now, requestID))

	status := createTestStatus(d, scheduler.StatusRunning, 12345)
	require.NoError(t, te.JSONDB.Write(status))

	// Clean up
	require.NoError(t, te.JSONDB.Close())

	// Verify
	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, statusFiles, 1)
	assert.Equal(t, status.RequestID, statusFiles[0].Status.RequestID)
	assert.Equal(t, status.Status, statusFiles[0].Status.Status)
	assert.Equal(t, status.PID, statusFiles[0].Status.PID)
}

func TestJSONDB_ReadStatusRecent(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_read_status_recent", "test_read_status_recent.yaml")

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
		writeTestStatus(t, te.JSONDB, d, data.Status, data.Timestamp)
	}

	recentStatus := te.JSONDB.ReadStatusRecent(d.Location, 2)
	require.Len(t, recentStatus, 2)
	assert.Equal(t, "request-id-3", recentStatus[0].Status.RequestID)
	assert.Equal(t, "request-id-2", recentStatus[1].Status.RequestID)
}

func TestJSONDB_FindByRequestID(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_find_by_request_id", "test_find_by_request_id.yaml")

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
		writeTestStatus(t, te.JSONDB, d, data.Status, data.Timestamp)
	}

	t.Run("ExistingRequestID", func(t *testing.T) {
		status, err := te.JSONDB.FindByRequestID(d.Location, "request-id-2")
		require.NoError(t, err)
		require.Equal(t, "request-id-2", status.Status.RequestID)
	})

	t.Run("NonExistentRequestID", func(t *testing.T) {
		status, err := te.JSONDB.FindByRequestID(d.Location, "request-id-10000")
		require.Error(t, err)
		require.Nil(t, status)
	})
}

func TestJSONDB_RemoveOld(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_remove_old", "test_remove_old.yaml")

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
		writeTestStatus(t, te.JSONDB, d, data.Status, data.Timestamp)
	}

	files := te.JSONDB.ReadStatusRecent(d.Location, 3)
	require.Equal(t, 3, len(files))

	// modify the timestamp of the first two files to be older than 1 day
	require.NoError(t, os.Chtimes(files[0].File, time.Now().AddDate(0, 0, -2), time.Now().AddDate(0, 0, -2)))
	require.NoError(t, os.Chtimes(files[1].File, time.Now().AddDate(0, 0, -1), time.Now().AddDate(0, 0, -1)))

	require.NoError(t, te.JSONDB.RemoveOld(d.Location, 1))

	files = te.JSONDB.ReadStatusRecent(d.Location, 3)
	require.Equal(t, 1, len(files))
}

func TestJSONDB_ReadStatusToday(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_read_status_today", "test_read_status_today.yaml")
	requestID := "request-id-1"
	now := time.Now()

	status := createTestStatus(d, scheduler.StatusRunning, 12345)
	status.RequestID = requestID
	writeTestStatus(t, te.JSONDB, d, status, now)

	te.JSONDB.latestStatusToday = true
	ret, err := te.JSONDB.ReadStatusToday(d.Location)

	require.NoError(t, err)
	require.NotNil(t, ret)
	require.Equal(t, status.RequestID, ret.RequestID)
	require.Equal(t, status.Status, ret.Status)
	require.Equal(t, status.PID, ret.PID)
}

func TestJSONDB_Update(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_update", "test_update.yaml")
	requestID := "request-id-1"
	now := time.Now()

	initialStatus := createTestStatus(d, scheduler.StatusRunning, 12345)
	initialStatus.RequestID = requestID
	writeTestStatus(t, te.JSONDB, d, initialStatus, now)

	updatedStatus := createTestStatus(d, scheduler.StatusSuccess, 12345)
	updatedStatus.RequestID = requestID
	require.NoError(t, te.JSONDB.Update(d.Location, requestID, updatedStatus))

	// Verify
	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, statusFiles, 1)
	assert.Equal(t, updatedStatus.RequestID, statusFiles[0].Status.RequestID)
	assert.Equal(t, updatedStatus.Status, statusFiles[0].Status.Status)
	assert.Equal(t, updatedStatus.PID, statusFiles[0].Status.PID)
}

func TestJSONDB_Compact(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_compact", "test_compact.yaml")
	requestID := "req-1"
	now := time.Now()

	status1 := createTestStatus(d, scheduler.StatusRunning, 12345)
	status1.RequestID = requestID
	writeTestStatus(t, te.JSONDB, d, status1, now)

	status2 := createTestStatus(d, scheduler.StatusSuccess, 12345)
	status2.RequestID = requestID
	require.NoError(t, te.JSONDB.Update(d.Location, requestID, status2))

	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, statusFiles, 1)

	// Verify compacted file
	compactedFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, compactedFiles, 1)
	assert.True(t, strings.HasSuffix(compactedFiles[0].File, "_c.dat"))
	assert.Equal(t, status2.Status, compactedFiles[0].Status.Status)
}

func TestJSONDB_Rename(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	oldID := "old_dag"
	newID := "new_dag"
	d := createTestDAG(te, oldID, oldID+".yaml")

	status := createTestStatus(d, scheduler.StatusSuccess, 12345)
	status.RequestID = "request-id-1"
	writeTestStatus(t, te.JSONDB, d, status, time.Now())

	oldPath := d.Location
	newPath := filepath.Join(filepath.Dir(d.Location), newID+".yaml")

	require.NoError(t, te.JSONDB.Rename(oldPath, newPath))

	// Check that old files are gone and new files exist
	oldIndexDir := filepath.Join(te.TmpDir, "index", oldID)
	assert.NoDirExists(t, oldIndexDir)

	newIndexDir := filepath.Join(te.TmpDir, "index", newID)
	assert.DirExists(t, newIndexDir)

	// Verify content of new files
	statusFiles := te.JSONDB.ReadStatusRecent(newPath, 1)
	require.Len(t, statusFiles, 1)
	assert.Equal(t, status.RequestID, statusFiles[0].Status.RequestID)
	assert.Equal(t, status.Status, statusFiles[0].Status.Status)
}

func TestParseStatusFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_parse", "test_parse.yaml")
	requestID := "request-id-1"
	now := time.Now()

	status := createTestStatus(d, scheduler.StatusRunning, 12345)
	status.RequestID = requestID
	writeTestStatus(t, te.JSONDB, d, status, now)

	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, statusFiles, 1)

	parsedStatus, err := ParseStatusFile(statusFiles[0].File)
	require.NoError(t, err)
	assert.Equal(t, status.RequestID, parsedStatus.RequestID)
	assert.Equal(t, status.Status, parsedStatus.Status)
	assert.Equal(t, status.PID, parsedStatus.PID)
}

func TestParseStatusFile_InvalidFile(t *testing.T) {
	_, err := ParseStatusFile("non_existent_file.dat")
	require.Error(t, err)
}

func TestParseStatusFile_EmptyFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	emptyFile := filepath.Join(te.TmpDir, "empty.dat")
	_, err := os.Create(emptyFile)
	require.NoError(t, err)

	_, err = ParseStatusFile(emptyFile)
	require.Error(t, err)
	assert.Equal(t, io.EOF, err)
}

func TestJSONDB_ReadStatusToday_NoData(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_no_data", "test_no_data.yaml")

	_, err := te.JSONDB.ReadStatusToday(d.Location)
	require.Error(t, err)
}

func TestJSONDB_FindByRequestID_EmptyID(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_empty_id", "test_empty_id.yaml")

	_, err := te.JSONDB.FindByRequestID(d.Location, "")
	require.Error(t, err)
}

func TestJSONDB_RemoveAll(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_remove_all", "test_remove_all.yaml")

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
		writeTestStatus(t, te.JSONDB, d, data.Status, data.Timestamp)
	}

	files := te.JSONDB.ReadStatusRecent(d.Location, 3)
	require.Equal(t, 3, len(files))

	require.NoError(t, te.JSONDB.RemoveAll(d.Location))

	files = te.JSONDB.ReadStatusRecent(d.Location, 3)
	require.Empty(t, files)
}

func TestJSONDB_Rename_Conflict(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	oldID := "old_dag"
	newID := "new_dag"
	d1 := createTestDAG(te, oldID, oldID+".yaml")
	d2 := createTestDAG(te, newID, newID+".yaml")

	status1 := createTestStatus(d1, scheduler.StatusSuccess, 12345)
	status1.RequestID = "request-id-1"
	writeTestStatus(t, te.JSONDB, d1, status1, time.Now())

	status2 := createTestStatus(d2, scheduler.StatusSuccess, 12346)
	status2.RequestID = "request-id-2"
	writeTestStatus(t, te.JSONDB, d2, status2, time.Now())

	err := te.JSONDB.Rename(d1.Location, d2.Location)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict")
}

func TestJSONDB_listRecentFiles(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_list_recent", "test_list_recent.yaml")

	// Create test data across multiple days
	for i := 0; i < 5; i++ {
		status := createTestStatus(d, scheduler.StatusNone, 10000+i)
		status.RequestID = fmt.Sprintf("request-id-%d", i+1)
		writeTestStatus(t, te.JSONDB, d, status, time.Now().AddDate(0, 0, -i))
	}

	files, err := te.JSONDB.listRecentFiles(filepath.Join(te.TmpDir, "status"), 3)
	require.NoError(t, err)
	assert.Len(t, files, 3)

	// Verify files are in reverse chronological order
	for i := 0; i < len(files)-1; i++ {
		assert.True(t, files[i] > files[i+1], "Files should be in reverse chronological order")
	}
}

func TestJSONDB_listDirsSorted(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	testDir := filepath.Join(te.TmpDir, "test_list_dirs")
	require.NoError(t, os.MkdirAll(testDir, 0755))

	// Create test directories
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
}

func TestJSONDB_listFilesSorted(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	testDir := filepath.Join(te.TmpDir, "test_list_files")
	require.NoError(t, os.MkdirAll(testDir, 0755))

	// Create test files
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
}

func TestJSONDB_latestToday(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_latest_today", "test_latest_today.yaml")
	now := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)

	// Create multiple status entries for today
	for i := 0; i < 3; i++ {
		status := createTestStatus(d, scheduler.StatusNone, 10000+i)
		status.RequestID = fmt.Sprintf("req-%d", i+1)
		writeTestStatus(t, te.JSONDB, d, status, now.Add(time.Duration(i)*time.Hour))
	}

	latestFile, err := te.JSONDB.latestToday(d.Location, now, true)
	require.NoError(t, err)
	assert.Contains(t, latestFile, now.Format("20060102"))
	assert.Contains(t, latestFile, "req-3")
}

func TestJSONDB_craftCompactedFileName(t *testing.T) {
	originalFile := "/path/to/status_file.dat"
	compactedFile, err := craftCompactedFileName(originalFile)
	require.NoError(t, err)
	assert.Equal(t, "/path/to/status_file_c.dat", compactedFile)

	// Test with already compacted file
	_, err = craftCompactedFileName("/path/to/status_file_c.dat")
	require.Error(t, err)
}

func TestJSONDB_ReadStatusForDate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-jsondb")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	logger := logger.Default
	db := New(tmpDir, logger, true)

	dagID := "test-dag"
	date := time.Date(2023, 5, 15, 0, 0, 0, 0, time.UTC)

	// Create some test data
	for i := 0; i < 3; i++ {
		requestID := fmt.Sprintf("request-id-%d", i)
		timestamp := date.Add(time.Duration(i) * time.Hour)

		err := db.Open(dagID, timestamp, requestID)
		require.NoError(t, err)

		status := &model.Status{
			Name:      dagID,
			RequestID: requestID,
			StartedAt: timestamp.Format(time.RFC3339),
			Status:    scheduler.StatusRunning,
		}
		err = db.Write(status)
		require.NoError(t, err)

		err = db.Close()
		require.NoError(t, err)
	}

	// Test reading status for the date
	statusFiles, err := db.ReadStatusForDate(dagID, date)
	require.NoError(t, err)
	assert.Len(t, statusFiles, 3)

	// Check if status files are sorted by timestamp in descending order
	for i := 0; i < len(statusFiles)-1; i++ {
		t1, _ := time.Parse(time.RFC3339, statusFiles[i].Status.StartedAt)
		t2, _ := time.Parse(time.RFC3339, statusFiles[i+1].Status.StartedAt)
		assert.True(t, t1.After(t2))
	}

	// Test reading status for a date with no data
	emptyDate := date.AddDate(0, 0, 1)
	emptyStatusFiles, err := db.ReadStatusForDate(dagID, emptyDate)
	require.NoError(t, err)
	assert.Empty(t, emptyStatusFiles)
}

func TestJSONDB_ListRecentStatusAllDAGs(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	// Create multiple DAGs with different statuses
	dags := []struct {
		name     string
		location string
	}{
		{"dag1", "dag1.yaml"},
		{"dag2", "dag2.yaml"},
		{"dag3", "dag3.yaml"},
	}

	now := time.Now()

	// Create test data
	for i, d := range dags {
		dag := createTestDAG(te, d.name, d.location)
		for j := 0; j < 3; j++ {
			tm := now.Add(-time.Duration(i*3+j) * time.Hour)
			status := createTestStatus(dag, scheduler.Status(j), 10000+i*3+j)
			status.RequestID = fmt.Sprintf("req-%d-%d", i, j)
			status.StartedAt = tm.Format(time.RFC3339)
			writeTestStatus(t, te.JSONDB, dag, status, tm)
		}
	}

	// Test retrieving recent status across all DAGs
	recentStatus, err := te.JSONDB.ListRecentStatusAllDAGs(5)
	require.NoError(t, err)
	assert.Len(t, recentStatus, 5)

	// Verify the order and content of the results
	for i := 0; i < len(recentStatus)-1; i++ {
		// Check if the statuses are in descending order of start time
		assert.True(t, recentStatus[i].Status.StartedAt > recentStatus[i+1].Status.StartedAt)
	}

	// Test with a larger number than available statuses
	allStatus, err := te.JSONDB.ListRecentStatusAllDAGs(100)
	require.NoError(t, err)
	assert.Len(t, allStatus, 9) // 3 DAGs * 3 statuses each

	// Test with zero
	zeroStatus, err := te.JSONDB.ListRecentStatusAllDAGs(0)
	require.NoError(t, err)
	assert.Empty(t, zeroStatus)

	// Test error case: non-existent directory
	te.JSONDB.baseDir = "/non/existent/dir"
	_, err = te.JSONDB.ListRecentStatusAllDAGs(5)
	assert.Error(t, err)
}
