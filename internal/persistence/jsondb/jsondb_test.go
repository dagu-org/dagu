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
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
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
		JSONDB: New(tmpDir, true),
		TmpDir: tmpDir,
	}
}

func (te testEnv) cleanup() {
	_ = os.RemoveAll(te.TmpDir)
}

func createTestDAG(te testEnv, name, location string) *digraph.DAG {
	return &digraph.DAG{
		Name:     name,
		Location: filepath.Join(te.TmpDir, location),
	}
}

func createTestStatus(d *digraph.DAG, status scheduler.Status, pid int) *model.Status {
	return model.NewStatus(d, nil, status, pid, nil, nil)
}

func writeTestStatus(t *testing.T, db *JSONDB, d *digraph.DAG, status *model.Status, tm time.Time) {
	dw, _, err := db.newWriter(d.Location, tm, status.RequestID)
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer dw.close()
	require.NoError(t, dw.write(status))
}

func TestNewDataFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_new_data_file", "test_new_data_file.yaml")
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	requestID := "request-id-1"

	t.Run("ValidFile", func(t *testing.T) {
		f, err := te.JSONDB.newFile(d.Location, timestamp, requestID)
		require.NoError(t, err)
		p := util.ValidFilename(strings.TrimSuffix(filepath.Base(d.Location), filepath.Ext(d.Location)))
		assert.Regexp(t, p+".*"+p+"\\.20220101\\.00:00:00\\.000\\."+requestID[:8]+"\\.dat", f)
	})

	t.Run("EmptyLocation", func(t *testing.T) {
		_, err := te.JSONDB.newFile("", timestamp, requestID)
		assert.Error(t, err)
	})
}

func TestWriteAndFindFiles(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_read_status_n", "test_data_files_n.yaml")

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

	files := te.JSONDB.latest(te.JSONDB.prefixWithDirectory(d.Location)+"*.dat", 2)
	require.Equal(t, 2, len(files))
}

func TestWriteAndFindByRequestID(t *testing.T) {
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

func TestRemoveOldFiles(t *testing.T) {
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

	files := te.JSONDB.latest(te.JSONDB.prefixWithDirectory(d.Location)+"*.dat", 3)
	require.Equal(t, 3, len(files))

	require.NoError(t, te.JSONDB.RemoveOld(d.Location, 0))

	files = te.JSONDB.latest(te.JSONDB.prefixWithDirectory(d.Location)+"*.dat", 3)
	require.Empty(t, files)

	invalidFiles := te.JSONDB.latest("invalid-pattern", 3)
	require.Empty(t, invalidFiles)
}

func TestReadLatestStatus(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_read_latest_status", "test_config_status_reader.yaml")
	requestID := "request-id-1"

	dw, _, err := te.JSONDB.newWriter(d.Location, time.Now(), requestID)
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer dw.close()

	initialStatus := createTestStatus(d, scheduler.StatusNone, 10000)
	require.NoError(t, dw.write(initialStatus))

	updatedStatus := createTestStatus(d, scheduler.StatusSuccess, 20000)
	require.NoError(t, dw.write(updatedStatus))

	ret, err := te.JSONDB.ReadStatusToday(d.Location)

	require.NoError(t, err)
	require.NotNil(t, ret)
	require.Equal(t, int(updatedStatus.PID), int(ret.PID))
	require.Equal(t, updatedStatus.Status, ret.Status)
}

func TestReadStatusN(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_read_status_n", "test_config_status_reader_hist.yaml")

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

	recordMax := 2

	ret := te.JSONDB.ReadStatusRecent(d.Location, recordMax)

	require.Len(t, ret, recordMax)
	require.Equal(t, d.Name, ret[0].Status.Name)
	require.Equal(t, d.Name, ret[1].Status.Name)
}

func TestCompactFile(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_compact_file", "test_compact_file.yaml")
	requestID := "request-id-1"

	dw, _, err := te.JSONDB.newWriter(d.Location, time.Now(), requestID)
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer dw.close()

	testStatuses := []scheduler.Status{scheduler.StatusRunning, scheduler.StatusCancel, scheduler.StatusSuccess}
	for _, status := range testStatuses {
		require.NoError(t, dw.write(createTestStatus(d, status, 10000)))
	}

	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.NotEmpty(t, statusFiles)
	s := statusFiles[0]

	db2 := New(te.JSONDB.location, true)
	require.NoError(t, db2.Compact(s.File))
	require.False(t, util.FileExists(s.File))

	compactedStatusFiles := db2.ReadStatusRecent(d.Location, 1)
	require.NotEmpty(t, compactedStatusFiles)
	s2 := compactedStatusFiles[0]

	require.Regexp(t, `test_compact_file.*_c.dat`, s2.File)
	require.Equal(t, s.Status, s2.Status)

	require.Error(t, db2.Compact("Invalid_file_name.dat"))
}

func TestErrorCases(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	t.Run("InvalidParseFile", func(t *testing.T) {
		_, err := ParseFile("invalid_file.dat")
		require.Error(t, err)
	})

	t.Run("InvalidNewWriter", func(t *testing.T) {
		_, _, err := te.JSONDB.newWriter("", time.Now(), "")
		require.Error(t, err)
	})

	t.Run("InvalidReadStatusToday", func(t *testing.T) {
		_, err := te.JSONDB.ReadStatusToday("invalid_file.yaml")
		require.Error(t, err)
	})

	t.Run("InvalidFindByRequestID", func(t *testing.T) {
		_, err := te.JSONDB.FindByRequestID("invalid_file.yaml", "invalid_id")
		require.Error(t, err)
	})
}

func TestErrorParseFile(t *testing.T) {
	tmpDir := util.MustTempDir("test_error_parse_file")
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "test_error_parse_file.dat")

	t.Run("NonExistentFile", func(t *testing.T) {
		_, err := ParseFile(tmpFile)
		require.Error(t, err)
	})

	f, err := util.OpenOrCreateFile(tmpFile)
	require.NoError(t, err)
	defer f.Close()

	t.Run("EmptyFile", func(t *testing.T) {
		_, err := ParseFile(tmpFile)
		require.Error(t, err)
	})

	t.Run("InvalidJSON", func(t *testing.T) {
		_, err = f.WriteString("invalid jsondb")
		require.NoError(t, err)
		_, err := ParseFile(tmpFile)
		require.Error(t, err)
	})

	t.Run("ValidJSON", func(t *testing.T) {
		_, err = f.WriteString("\n{}")
		require.NoError(t, err)
		_, err := ParseFile(tmpFile)
		require.NoError(t, err)
	})
}

func TestTimestamp(t *testing.T) {
	testCases := []struct {
		Name string
		Want string
	}{
		{Name: "test_timestamp.20200101.10:00:00.dat", Want: "20200101.10:00:00"},
		{Name: "test_timestamp.20200101.12:34:56_c.dat", Want: "20200101.12:34:56"},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			require.Equal(t, tc.Want, timestamp(tc.Name))
		})
	}
}

func TestReadLine(t *testing.T) {
	tmpDir := util.MustTempDir("test_read_line")
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "test_read_line.dat")

	f, err := util.OpenOrCreateFile(tmpFile)
	require.NoError(t, err)
	defer f.Close()

	t.Run("EmptyFile", func(t *testing.T) {
		_, err = readLineFrom(f, 0)
		require.Error(t, err)
	})

	dat := []byte("line1\nline2")
	_, err = f.Write(dat)
	require.NoError(t, err)
	require.NoError(t, f.Sync())

	f, err = os.Open(tmpFile)
	require.NoError(t, err)
	defer f.Close()

	t.Run("ReadLines", func(t *testing.T) {
		_, err = f.Seek(0, 0)
		require.NoError(t, err)

		testCases := []struct {
			Want []byte
		}{
			{Want: []byte("line1")},
			{Want: []byte("line2")},
		}

		var offset int64
		for i, tc := range testCases {
			got, err := readLineFrom(f, offset)
			require.NoError(t, err)
			require.Equal(t, tc.Want, got, "Line %d mismatch", i+1)
			offset += int64(len(tc.Want)) + 1 // +1 for \n
		}

		_, err = readLineFrom(f, offset)
		require.Equal(t, io.EOF, err)
	})
}

func TestRename(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	oldID := "old_dag"
	newID := "new_dag"
	d := createTestDAG(te, oldID, oldID+".yaml")

	// Create some test data
	testData := []struct {
		Status    *model.Status
		ReqID     string
		Timestamp time.Time
	}{
		{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-1", time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)},
		{createTestStatus(d, scheduler.StatusNone, 10000), "request-id-2", time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local)},
	}

	for _, data := range testData {
		data.Status.RequestID = data.ReqID
		writeTestStatus(t, te.JSONDB, d, data.Status, data.Timestamp)
	}

	oldPath := d.Location
	newPath := filepath.Join(filepath.Dir(d.Location), newID+".yaml")

	// Perform rename
	err := te.JSONDB.Rename(d.Location, filepath.Join(filepath.Dir(d.Location), newID+".yaml"))
	require.NoError(t, err)

	// Check that old files are gone and new files exist
	oldDir := te.JSONDB.getDirectory(oldPath, oldID)
	oldFiles, err := filepath.Glob(filepath.Join(oldDir, "*"))
	require.NoError(t, err)
	require.Empty(t, oldFiles)

	newDir := te.JSONDB.getDirectory(newPath, newID)
	newFiles, err := filepath.Glob(filepath.Join(newDir, "*"))
	require.NoError(t, err)
	require.Len(t, newFiles, 2)

	// Verify content of new files
	d.Name = newID
	d.Location = newPath
	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 2)
	require.Len(t, statusFiles, 2)
	for i, sf := range statusFiles {
		require.Equal(t, testData[1-i].ReqID, sf.Status.RequestID) // Reverse order due to recent first
	}
}

func TestNewJSONDB(t *testing.T) {
	location := "/tmp/jsondb"
	latestStatusToday := true

	db := New(location, latestStatusToday)

	require.NotNil(t, db)
	require.Equal(t, location, db.location)
	require.Equal(t, latestStatusToday, db.latestStatusToday)
	require.NotNil(t, db.cache)
}

func TestJSONDBOpen(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_open", "test_open.yaml")
	requestID := "request-id-1"
	now := time.Now()

	err := te.JSONDB.Open(d.Location, now, requestID)
	require.NoError(t, err)
	require.NotNil(t, te.JSONDB.writer)

	// Clean up
	require.NoError(t, te.JSONDB.Close())
}

func TestJSONDBWrite(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	d := createTestDAG(te, "test_write", "test_write.yaml")
	requestID := "request-id-1"
	now := time.Now()

	require.NoError(t, te.JSONDB.Open(d.Location, now, requestID))

	status := createTestStatus(d, scheduler.StatusRunning, 12345)
	require.NoError(t, te.JSONDB.Write(status))

	// Clean up
	require.NoError(t, te.JSONDB.Close())

	// Verify
	statusFiles := te.JSONDB.ReadStatusRecent(d.Location, 1)
	require.Len(t, statusFiles, 1)
	require.Equal(t, status.RequestID, statusFiles[0].Status.RequestID)
	require.Equal(t, status.Status, statusFiles[0].Status.Status)
	require.Equal(t, status.PID, statusFiles[0].Status.PID)
}

func TestJSONDBUpdate(t *testing.T) {
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
	require.Equal(t, updatedStatus.RequestID, statusFiles[0].Status.RequestID)
	require.Equal(t, updatedStatus.Status, statusFiles[0].Status.Status)
	require.Equal(t, updatedStatus.PID, statusFiles[0].Status.PID)
}
