package jsondb

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/utils"
	"github.com/stretchr/testify/require"
)

func setupTest(t *testing.T) (string, *Store) {
	tmpDir, err := os.MkdirTemp("", "test-persistence")
	require.NoError(t, err)
	db := New(tmpDir, "")
	return tmpDir, db
}

func TestNewDataFile(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{Location: "test_new_data_file.yaml"}
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	requestId := "request-id-1"
	f, err := db.newFile(d.Location, timestamp, requestId)
	require.NoError(t, err)
	p := utils.ValidFilename(strings.TrimSuffix(
		path.Base(d.Location), path.Ext(d.Location)), "_")
	require.Regexp(t, fmt.Sprintf("%s.*/%s.20220101.00:00:00.000.%s.dat", p, p, requestId[:8]), f)

	_, err = db.newFile("", timestamp, requestId)
	require.Error(t, err)
}

func TestWriteAndFindFiles(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{
		Name:     "test_read_status_n",
		Location: "test_data_files_n.yaml",
	}

	for _, data := range []struct {
		Status    *model.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, status, data.Timestamp)
	}

	files := db.latest(db.pattern(d.Location)+"*.dat", 2)
	require.Equal(t, 2, len(files))
}

func TestWriteAndFindByRequestId(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{
		Name:     "test_find_by_request_id",
		Location: "test_find_by_request_id.yaml",
	}

	defer func() {
		_ = db.RemoveAll(d.Location)
	}()

	for _, data := range []struct {
		Status    *model.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, status, data.Timestamp)
	}

	status, err := db.FindByRequestId(d.Location, "request-id-2")
	require.NoError(t, err)
	require.Equal(t, status.Status.RequestId, "request-id-2")

	status, err = db.FindByRequestId(d.Location, "request-id-10000")
	require.Error(t, err)
	require.Nil(t, status)
}

func TestRemoveOldFiles(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{Location: "test_remove_old.yaml"}

	for _, data := range []struct {
		Status    *model.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, data.Status, data.Timestamp)
	}

	files := db.latest(db.pattern(d.Location)+"*.dat", 3)
	require.Equal(t, 3, len(files))

	_ = db.RemoveOld(d.Location, 0)

	files = db.latest(db.pattern(d.Location)+"*.dat", 3)
	require.Equal(t, 0, len(files))

	m := db.latest("invalid-pattern", 3)
	require.Equal(t, 0, len(m))
}

func TestReadLatestStatus(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{Location: "test_config_status_reader.yaml"}
	requestId := "request-id-1"

	dw, _, err := db.newWriter(d.Location, time.Now(), requestId)
	require.NoError(t, err)
	err = dw.open()
	require.NoError(t, err)
	defer func() {
		_ = dw.close()
	}()

	status := model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil)
	err = dw.write(status)
	require.NoError(t, err)

	status.Status = scheduler.SchedulerStatus_Success
	status.Pid = 20000
	_ = dw.write(status)

	ret, err := db.ReadStatusToday(d.Location)

	require.NoError(t, err)
	require.NotNil(t, ret)
	require.Equal(t, int(status.Pid), int(ret.Pid))
	require.Equal(t, status.Status, ret.Status)

}

func TestReadStatusN(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{Name: "test_read_status_n", Location: "test_config_status_reader_hist.yaml"}

	for _, data := range []struct {
		Status    *model.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			model.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, data.Status, data.Timestamp)
	}

	recordMax := 2

	ret := db.ReadStatusRecent(d.Location, recordMax)

	require.Equal(t, recordMax, len(ret))
	require.Equal(t, d.Name, ret[0].Status.Name)
	require.Equal(t, d.Name, ret[1].Status.Name)
}

func TestCompactFile(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	d := &dag.DAG{Name: "test_compact_file", Location: "test_compact_file.yaml"}
	requestId := "request-id-1"

	dw, _, err := db.newWriter(d.Location, time.Now(), requestId)
	require.NoError(t, err)
	require.NoError(t, dw.open())

	for _, data := range []struct {
		Status *model.Status
	}{
		{model.NewStatus(
			d, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)},
		{model.NewStatus(
			d, nil, scheduler.SchedulerStatus_Cancel, 10000, nil, nil)},
		{model.NewStatus(
			d, nil, scheduler.SchedulerStatus_Success, 10000, nil, nil)},
	} {
		require.NoError(t, dw.write(data.Status))
	}

	_ = dw.close()

	var s *model.StatusFile = nil
	if h := db.ReadStatusRecent(d.Location, 1); len(h) > 0 {
		s = h[0]
	}
	require.NotNil(t, s)

	db2 := New(db.dir, "")
	err = db2.Compact(d.Location, s.File)
	require.False(t, utils.FileExists(s.File))
	require.NoError(t, err)

	var s2 *model.StatusFile = nil
	if h := db2.ReadStatusRecent(d.Location, 1); len(h) > 0 {
		s2 = h[0]
	}
	require.NotNil(t, s2)

	require.Regexp(t, `test_compact_file.*_c.dat`, s2.File)
	require.Equal(t, s.Status, s2.Status)

	err = db2.Compact(d.Location, "Invalid_file_name.dat")
	require.Error(t, err)
}

func TestErrorReadFile(t *testing.T) {
	tmpDir, db := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	_, err := ParseFile("invalid_file.dat")
	require.Error(t, err)

	_, _, err = db.newWriter("", time.Now(), "")
	require.Error(t, err)

	_, err = db.ReadStatusToday("invalid_file.yaml")
	require.Error(t, err)

	_, err = db.FindByRequestId("invalid_file.yaml", "invalid_id")
	require.Error(t, err)
}

func TestErrorParseFile(t *testing.T) {
	tmpDir := utils.MustTempDir("test_error_parse_file")
	tmpFile := filepath.Join(tmpDir, "test_error_parse_file.dat")

	_, err := ParseFile(tmpFile)
	require.Error(t, err)

	f, err := utils.OpenOrCreateFile(tmpFile)
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.Error(t, err)

	_, err = f.WriteString("invalid jsondb")
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.Error(t, err)

	_, err = f.WriteString("\n{}")
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.NoError(t, err)
}

func testWriteStatus(t *testing.T, db *Store, d *dag.DAG, status *model.Status, tm time.Time) {
	t.Helper()
	dw, _, err := db.newWriter(d.Location, tm, status.RequestId)
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer func() {
		_ = dw.close()
	}()
	require.NoError(t, dw.write(status))
}

func TestTimestamp(t *testing.T) {
	for _, tt := range []struct {
		Name string
		Want string
	}{
		{Name: "test_timestamp.20200101.10:00:00.dat", Want: "20200101.10:00:00"},
		{Name: "test_timestamp.20200101.12:34:56_c.dat", Want: "20200101.12:34:56"},
	} {
		require.Equal(t, tt.Want, timestamp(tt.Name))
	}
}

func TestReadLine(t *testing.T) {
	tmpDir := utils.MustTempDir("test_read_line")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()
	tmpFile := filepath.Join(tmpDir, "test_read_line.dat")

	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)

	// error
	_, err = readLineFrom(f, 0)
	require.Error(t, err)

	// write data
	dat := []byte("line1\nline2")
	_, _ = f.Write(dat)

	err = f.Sync()
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	f, err = os.Open(tmpFile)
	require.NoError(t, err)

	_, _ = f.Seek(0, 0)
	var offset int64 = 0
	for _, tt := range []struct {
		Want []byte
	}{
		{Want: []byte("line1")},
		{Want: []byte("line2")},
	} {
		got, err := readLineFrom(f, offset)
		require.NoError(t, err)
		require.Equal(t, tt.Want, got)
		offset += int64(len([]byte(tt.Want))) + 1 // +1 for \n
	}
	_, err = readLineFrom(f, offset)
	require.Equal(t, io.EOF, err)
}
