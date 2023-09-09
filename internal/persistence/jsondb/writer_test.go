package jsondb

import (
	"fmt"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"os"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/stretchr/testify/require"
)

func testWriteStatusToFile(t *testing.T, db *Store) {
	d := &dag.DAG{
		Name:     "test_write_status",
		Location: "test_write_status.yaml",
	}
	dw, file, err := db.newWriter(d.Location, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.open())
	defer func() {
		_ = dw.close()
		_ = db.RemoveOld(d.Location, 0)
	}()

	status := model.NewStatus(d, nil, scheduler.SchedulerStatus_Running, 10000, nil, nil)
	status.RequestId = fmt.Sprintf("request-id-%d", time.Now().Unix())
	require.NoError(t, dw.write(status))
	require.Regexp(t, ".*test_write_status.*", file)

	dat, err := os.ReadFile(file)
	require.NoError(t, err)

	r, err := model.StatusFromJson(string(dat))
	require.NoError(t, err)

	require.Equal(t, d.Name, r.Name)

	err = dw.close()
	require.NoError(t, err)

	old := d.Location
	new := "text_write_status_new.yaml"

	oldDir := db.directory(old, prefix(old))
	newDir := db.directory(new, prefix(new))
	require.DirExists(t, oldDir)
	require.NoDirExists(t, newDir)

	err = db.Rename(old, new)
	require.NoError(t, err)
	require.NoDirExists(t, oldDir)
	require.DirExists(t, newDir)

	ret := db.ReadStatusHist(new, 1)
	require.Equal(t, 1, len(ret))
	require.Equal(t, status.RequestId, ret[0].Status.RequestId)
}

func testWriteStatusToExistingFile(t *testing.T, db *Store) {
	d := &dag.DAG{
		Name:     "test_append_to_existing",
		Location: "test_append_to_existing.yaml",
	}
	dw, file, err := db.newWriter(d.Location, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.open())

	status := model.NewStatus(d, nil, scheduler.SchedulerStatus_Cancel, 10000, nil, nil)
	status.RequestId = "request-id-test-write-status-to-existing-file"
	require.NoError(t, dw.write(status))
	dw.close()

	data, err := db.FindByRequestId(d.Location, status.RequestId)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Cancel)
	require.Equal(t, file, data.File)

	dw = &writer{target: file}
	require.NoError(t, dw.open())
	status.Status = scheduler.SchedulerStatus_Success
	require.NoError(t, dw.write(status))
	dw.close()

	data, err = db.FindByRequestId(d.Location, status.RequestId)
	require.NoError(t, err)
	require.Equal(t, data.Status.Status, scheduler.SchedulerStatus_Success)
	require.Equal(t, file, data.File)
}
