package database

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
)

func testWriteStatusToFile(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_write_status",
		ConfigPath: "test_write_status.yaml",
	}
	dw, file, err := db.NewWriter(cfg.ConfigPath, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.Open())
	defer func() {
		dw.Close()
		db.RemoveOld(db.pattern(cfg.ConfigPath)+"*.dat", 0)
	}()

	status := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)
	status.RequestId = fmt.Sprintf("request-id-%d", time.Now().Unix())
	require.NoError(t, dw.Write(status))
	require.Regexp(t, ".*test_write_status.*", file)

	dat, err := os.ReadFile(file)
	require.NoError(t, err)

	r, err := models.StatusFromJson(string(dat))
	require.NoError(t, err)

	require.Equal(t, cfg.Name, r.Name)

	err = dw.Close()
	require.NoError(t, err)

	old := cfg.ConfigPath
	new := "text_write_status_new.yaml"

	oldDir := db.dir(old, prefix(old))
	newDir := db.dir(new, prefix(new))
	require.DirExists(t, oldDir)
	require.NoDirExists(t, newDir)

	err = db.MoveData(old, new)
	require.NoError(t, err)
	require.NoDirExists(t, oldDir)
	require.DirExists(t, newDir)

	ret := db.ReadStatusHist(new, 1)
	require.Equal(t, 1, len(ret))
	require.Equal(t, status.RequestId, ret[0].Status.RequestId)
}

func testWriteStatusToExistingFile(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_append_to_existing",
		ConfigPath: "test_append_to_existing.yaml",
	}
	dw, file, err := db.NewWriter(cfg.ConfigPath, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.Open())

	status := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_Cancel, 10000, nil, nil)
	status.RequestId = "request-id-test-write-status-to-existing-file"
	require.NoError(t, dw.Write(status))
	dw.Close()

	data, err := db.FindByRequestId(cfg.ConfigPath, status.RequestId)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Cancel)
	require.Equal(t, file, data.File)

	dw = &Writer{Target: file}
	require.NoError(t, dw.Open())
	status.Status = scheduler.SchedulerStatus_Success
	require.NoError(t, dw.Write(status))
	dw.Close()

	data, err = db.FindByRequestId(cfg.ConfigPath, status.RequestId)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Success)
	require.Equal(t, file, data.File)
}
