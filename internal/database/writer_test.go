package database

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagman/internal/config"
	"github.com/yohamta/dagman/internal/models"
	"github.com/yohamta/dagman/internal/scheduler"
	"github.com/yohamta/dagman/internal/utils"
)

func testWriteStatusToFile(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_write_status",
		ConfigPath: "test_write_status.yaml",
	}
	dw, file, err := db.NewWriter(cfg.ConfigPath, time.Now())
	require.NoError(t, err)
	require.NoError(t, dw.Open())
	defer func() {
		dw.Close()
		db.RemoveOld(cfg.ConfigPath, 0)
	}()

	status := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)
	require.NoError(t, dw.Write(status))

	utils.AssertPattern(t, "FileName", ".*test_write_status.*", file)

	dat, err := os.ReadFile(file)
	require.NoError(t, err)

	r, err := models.StatusFromJson(string(dat))
	require.NoError(t, err)

	assert.Equal(t, cfg.Name, r.Name)
}

func testWriteStatusToExistingFile(t *testing.T, db *Database) {
	cfg := &config.Config{
		Name:       "test_append_to_existing",
		ConfigPath: "test_append_to_existing.yaml",
	}
	dw, file, err := db.NewWriter(cfg.ConfigPath, time.Now())
	require.NoError(t, err)
	require.NoError(t, dw.Open())

	status := models.NewStatus(cfg, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)
	status.RequestId = "request-id-test-write-status-to-existing-file"
	require.NoError(t, dw.Write(status))
	dw.Close()

	data, err := db.FindByRequestId(cfg.ConfigPath, status.RequestId)
	require.NoError(t, err)
	assert.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Running)
	assert.Equal(t, file, data.File)

	dw, err = db.NewWriterFor(cfg.ConfigPath, file)
	require.NoError(t, err)
	require.NoError(t, dw.Open())
	status.Status = scheduler.SchedulerStatus_Success
	require.NoError(t, dw.Write(status))
	dw.Close()

	data, err = db.FindByRequestId(cfg.ConfigPath, status.RequestId)
	require.NoError(t, err)
	assert.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Success)
	assert.Equal(t, file, data.File)
}
