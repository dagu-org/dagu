package database

import (
	"fmt"
	"io/ioutil"
	"jobctl/internal/config"
	"jobctl/internal/models"
	"jobctl/internal/scheduler"
	"jobctl/internal/utils"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDatabase(t *testing.T) {
	for scenario, fn := range map[string]func(
		t *testing.T, db *Database,
	){
		"create new datafile":                 testNewDataFile,
		"write status to file":                testWriteStatusToFile,
		"append status to existing file":      testWriteStatusToExistingFile,
		"write status and find files":         testWriteAndFindFiles,
		"write status and find by request id": testWriteAndFindByRequestId,
		"remove old files":                    testRemoveOldFiles,
		"test read latest status":             testReadLatestStatus,
		"test read latest n status":           testReadStatusN,
	} {
		t.Run(scenario, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "test-database")
			db := New(&Config{
				Dir: dir,
			})
			require.NoError(t, err)
			defer os.RemoveAll(dir)
			fn(t, db)
		})
	}
}

func testNewDataFile(t *testing.T, db *Database) {
	cfg := &config.Config{
		ConfigPath: "test_new_data_file.yaml",
	}
	timestamp := time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local)
	f, err := db.new(cfg.ConfigPath, timestamp)
	require.NoError(t, err)
	p := utils.ValidFilename(strings.TrimSuffix(
		path.Base(cfg.ConfigPath), path.Ext(cfg.ConfigPath)), "_")
	assert.Regexp(t, fmt.Sprintf("%s.*/%s.20220101.00:00:00.dat", p, p), f)
}

func testWriteAndFindFiles(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_read_status_n",
		ConfigPath: "test_data_files_n.yaml",
	}
	defer db.RemoveAll(cfg.ConfigPath)

	for _, data := range []struct {
		Status    *models.Status
		Timestamp time.Time
	}{
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		testWriteStatus(t, db, cfg, data.Status, data.Timestamp)
	}

	files, err := db.latest(cfg.ConfigPath, 2)
	require.NoError(t, err)
	require.Equal(t, 2, len(files))
}

func testWriteAndFindByRequestId(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_find_by_request_id",
		ConfigPath: "test_find_by_request_id.yaml",
	}
	defer db.RemoveAll(cfg.ConfigPath)

	for _, data := range []struct {
		Status    *models.Status
		RequestId string
		Timestamp time.Time
	}{
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			fmt.Sprintf("request-id-1"),
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			fmt.Sprintf("request-id-2"),
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			fmt.Sprintf("request-id-3"),
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		status := data.Status
		status.RequestId = data.RequestId
		testWriteStatus(t, db, cfg, status, data.Timestamp)
	}

	status, err := db.FindByRequestId(cfg.ConfigPath, "request-id-2")
	require.NoError(t, err)
	assert.Equal(t, status.Status.RequestId, "request-id-2")

	status, err = db.FindByRequestId(cfg.ConfigPath, "request-id-10000")
	assert.Error(t, err)
	assert.Nil(t, status)
}

func testRemoveOldFiles(t *testing.T, db *Database) {
	cfg := &config.Config{
		ConfigPath: "test_remove_old.yaml",
	}

	for _, data := range []struct {
		Status    *models.Status
		Timestamp time.Time
	}{
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		testWriteStatus(t, db, cfg, data.Status, data.Timestamp)
	}

	files, err := db.latest(cfg.ConfigPath, 3)
	require.NoError(t, err)
	require.Equal(t, 3, len(files))

	db.RemoveOld(cfg.ConfigPath, 0)

	files, err = db.latest(cfg.ConfigPath, 3)
	require.Equal(t, err, ErrNoDataFile)
	require.Equal(t, 0, len(files))
}

func testReadLatestStatus(t *testing.T, db *Database) {
	cfg := &config.Config{
		ConfigPath: "test_config_status_reader.yaml",
	}
	dw, _, err := db.NewWriter(cfg.ConfigPath, time.Now())
	require.NoError(t, err)
	err = dw.Open()
	require.NoError(t, err)
	defer dw.Close()

	status := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil)
	dw.Write(status)

	status.Status = scheduler.SchedulerStatus_Running
	status.Pid = 20000
	dw.Write(status)

	ret, err := db.ReadStatusToday(cfg.ConfigPath)

	require.NoError(t, err)
	require.NotNil(t, ret)
	assert.Equal(t, int(status.Pid), int(ret.Pid))
	require.Equal(t, status.Status, ret.Status)
}

func testReadStatusN(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_read_status_n",
		ConfigPath: "test_config_status_reader_hist.yaml",
	}

	for _, data := range []struct {
		Status    *models.Status
		Timestamp time.Time
	}{
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 1, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 2, 0, 0, 0, 0, time.Local),
		},
		{
			models.NewStatus(cfg, nil, scheduler.SchedulerStatus_None, 10000, nil, nil),
			time.Date(2022, 1, 3, 0, 0, 0, 0, time.Local),
		},
	} {
		testWriteStatus(t, db, cfg, data.Status, data.Timestamp)
	}

	recordMax := 2

	ret, err := db.ReadStatusHist(cfg.ConfigPath, recordMax)

	require.NoError(t, err)
	require.Equal(t, recordMax, len(ret))
	assert.Equal(t, cfg.Name, ret[0].Status.Name)
	assert.Equal(t, cfg.Name, ret[1].Status.Name)
}

func testWriteStatus(t *testing.T, db *Database, cfg *config.Config, status *models.Status, tm time.Time) {
	t.Helper()
	dw, _, err := db.NewWriter(cfg.ConfigPath, tm)
	require.NoError(t, err)
	require.NoError(t, dw.Open())
	defer dw.Close()
	require.NoError(t, dw.Write(status))
}
