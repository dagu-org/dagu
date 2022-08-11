package database

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

func TestDatabase(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, db *Database,
	){
		"create new datafile":                 testNewDataFile,
		"write status to file and rename":     testWriteStatusToFile,
		"append status to existing file":      testWriteStatusToExistingFile,
		"write status and find files":         testWriteAndFindFiles,
		"write status and find by request id": testWriteAndFindByRequestId,
		"remove old files":                    testRemoveOldFiles,
		"test read latest status":             testReadLatestStatus,
		"test read latest n status":           testReadStatusN,
		"test compaction":                     testCompactFile,
		"test error read file":                testErrorReadFile,
		"test error parse file":               testErrorParseFile,
	} {
		t.Run(scenario, func(t *testing.T) {
			dir, err := os.MkdirTemp("", "test-database")
			db := &Database{
				Config: &Config{
					Dir: dir,
				},
			}
			require.NoError(t, err)
			defer os.RemoveAll(dir)
			fn(t, db)
		})
	}
}

func testNewDataFile(t *testing.T, db *Database) {
	d := &dag.DAG{
		ConfigPath: "test_new_data_file.yaml",
	}
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	requestId := "request-id-1"
	f, err := db.newFile(d.ConfigPath, timestamp, requestId)
	require.NoError(t, err)
	p := utils.ValidFilename(strings.TrimSuffix(
		path.Base(d.ConfigPath), path.Ext(d.ConfigPath)), "_")
	require.Regexp(t, fmt.Sprintf("%s.*/%s.20220101.00:00:00.000.%s.dat", p, p, requestId[:8]), f)

	_, err = db.newFile("", timestamp, requestId)
	require.Error(t, err)
}

func testWriteAndFindFiles(t *testing.T, db *Database) {
	d := &dag.DAG{
		Name:       "test_read_status_n",
		ConfigPath: "test_data_files_n.yaml",
	}
	defer db.RemoveAll(d.ConfigPath)

	for _, data := range []struct {
		Status    *models.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, status, data.Timestamp)
	}

	files := db.latest(db.pattern(d.ConfigPath)+"*.dat", 2)
	require.Equal(t, 2, len(files))
}

func testWriteAndFindByRequestId(t *testing.T, db *Database) {
	d := &dag.DAG{
		Name:       "test_find_by_request_id",
		ConfigPath: "test_find_by_request_id.yaml",
	}
	defer db.RemoveAll(d.ConfigPath)

	for _, data := range []struct {
		Status    *models.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, status, data.Timestamp)
	}

	status, err := db.FindByRequestId(d.ConfigPath, "request-id-2")
	require.NoError(t, err)
	require.Equal(t, status.Status.RequestId, "request-id-2")

	status, err = db.FindByRequestId(d.ConfigPath, "request-id-10000")
	require.Error(t, err)
	require.Nil(t, status)
}

func testRemoveOldFiles(t *testing.T, db *Database) {
	d := &dag.DAG{
		ConfigPath: "test_remove_old.yaml",
	}

	for _, data := range []struct {
		Status    *models.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, data.Status, data.Timestamp)
	}

	files := db.latest(db.pattern(d.ConfigPath)+"*.dat", 3)
	require.Equal(t, 3, len(files))

	db.RemoveOld(d.ConfigPath, 0)

	files = db.latest(db.pattern(d.ConfigPath)+"*.dat", 3)
	require.Equal(t, 0, len(files))

	m := db.latest("invalid-pattern", 3)
	require.Equal(t, 0, len(m))
}

func testReadLatestStatus(t *testing.T, db *Database) {
	d := &dag.DAG{
		ConfigPath: "test_config_status_reader.yaml",
	}
	requestId := "request-id-1"
	dw, _, err := db.NewWriter(d.ConfigPath, time.Now(), requestId)
	require.NoError(t, err)
	err = dw.Open()
	require.NoError(t, err)
	defer dw.Close()

	status := models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil)
	dw.Write(status)

	status.Status = scheduler.SchedulerStatus_Success
	status.Pid = 20000
	dw.Write(status)

	ret, err := db.ReadStatusToday(d.ConfigPath)

	require.NoError(t, err)
	require.NotNil(t, ret)
	require.Equal(t, int(status.Pid), int(ret.Pid))
	require.Equal(t, status.Status, ret.Status)

}

func testReadStatusN(t *testing.T, db *Database) {
	d := &dag.DAG{
		Name:       "test_read_status_n",
		ConfigPath: "test_config_status_reader_hist.yaml",
	}

	for _, data := range []struct {
		Status    *models.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-1",
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-2",
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(d, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			"request-id-3",
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, d, data.Status, data.Timestamp)
	}

	recordMax := 2

	ret := db.ReadStatusHist(d.ConfigPath, recordMax)

	require.Equal(t, recordMax, len(ret))
	require.Equal(t, d.Name, ret[0].Status.Name)
	require.Equal(t, d.Name, ret[1].Status.Name)
}

func testCompactFile(t *testing.T, db *Database) {
	d := &dag.DAG{
		Name:       "test_compact_file",
		ConfigPath: "test_compact_file.yaml",
	}
	requestId := "request-id-1"

	dw, _, err := db.NewWriter(d.ConfigPath, time.Now(), requestId)
	require.NoError(t, err)
	require.NoError(t, dw.Open())

	for _, data := range []struct {
		Status *models.Status
	}{
		{models.NewStatus(
			d, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)},
		{models.NewStatus(
			d, nil, scheduler.SchedulerStatus_Cancel, 10000, nil, nil)},
		{models.NewStatus(
			d, nil, scheduler.SchedulerStatus_Success, 10000, nil, nil)},
	} {
		require.NoError(t, dw.Write(data.Status))
	}

	dw.Close()

	var s *models.StatusFile = nil
	if h := db.ReadStatusHist(d.ConfigPath, 1); len(h) > 0 {
		s = h[0]
	}
	require.NotNil(t, s)

	db2 := &Database{
		Config: db.Config,
	}
	err = db2.Compact(d.ConfigPath, s.File)
	require.False(t, utils.FileExists(s.File))
	require.NoError(t, err)

	var s2 *models.StatusFile = nil
	if h := db2.ReadStatusHist(d.ConfigPath, 1); len(h) > 0 {
		s2 = h[0]
	}
	require.NotNil(t, s2)

	require.Regexp(t, `test_compact_file.*_c.dat`, s2.File)
	require.Equal(t, s.Status, s2.Status)

	err = db2.Compact(d.ConfigPath, "Invalid_file_name.dat")
	require.Error(t, err)
}

func testErrorReadFile(t *testing.T, db *Database) {
	_, err := ParseFile("invalid_file.dat")
	require.Error(t, err)

	_, _, err = db.NewWriter("", time.Now(), "")
	require.Error(t, err)

	_, err = db.ReadStatusToday("invalid_file.yaml")
	require.Error(t, err)

	_, err = db.FindByRequestId("invalid_file.yaml", "invalid_id")
	require.Error(t, err)
}

func testErrorParseFile(t *testing.T, db *Database) {
	tmpDir := utils.MustTempDir("test_error_parse_file")
	tmpFile := filepath.Join(tmpDir, "test_error_parse_file.dat")

	_, err := ParseFile(tmpFile)
	require.Error(t, err)

	f, err := utils.OpenOrCreateFile(tmpFile)
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.Error(t, err)

	_, err = f.WriteString("invalid json")
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.Error(t, err)

	_, err = f.WriteString("\n{}")
	require.NoError(t, err)

	_, err = ParseFile(tmpFile)
	require.NoError(t, err)
}

func testWriteStatus(t *testing.T, db *Database, d *dag.DAG, status *models.Status, tm time.Time) {
	t.Helper()
	dw, _, err := db.NewWriter(d.ConfigPath, tm, status.RequestId)
	require.NoError(t, err)
	require.NoError(t, dw.Open())
	defer dw.Close()
	require.NoError(t, dw.Write(status))
}

func TestDefaultConfig(t *testing.T) {
	d := DefaultConfig()
	require.Equal(t, d.Dir, settings.MustGet(settings.SETTING__DATA_DIR))
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
	defer os.RemoveAll(tmpDir)
	tmpFile := filepath.Join(tmpDir, "test_read_line.dat")

	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY, 0644)
	require.NoError(t, err)

	// error
	_, err = readLineFrom(f, 0)
	require.Error(t, err)

	// write data
	dat := []byte("line1\nline2")
	f.Write(dat)

	err = f.Sync()
	require.NoError(t, err)

	err = f.Close()
	require.NoError(t, err)

	f, err = os.Open(tmpFile)
	require.NoError(t, err)

	f.Seek(0, 0)
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
