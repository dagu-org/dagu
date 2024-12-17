// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package jsondb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	t.Run("WriteStatusToNewFile", func(t *testing.T) {
		d := &digraph.DAG{
			Name:     "test_write_status",
			Location: "test_write_status.yaml",
		}
		requestID := fmt.Sprintf("request-id-%d", time.Now().Unix())

		dw, file, err := te.JSONDB.newWriter(d.Location, time.Now(), requestID)
		require.NoError(t, err)
		require.NoError(t, dw.open())

		defer func() {
			assert.NoError(t, dw.close())
			assert.NoError(t, te.JSONDB.RemoveOld(d.Location, 0))
		}()

		status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
		status.RequestID = requestID
		require.NoError(t, dw.write(status))
		assert.Regexp(t, ".*test_write_status.*", file)

		// Verify file contents
		dat, err := os.ReadFile(file)
		require.NoError(t, err)

		r, err := model.StatusFromJSON(string(dat))
		require.NoError(t, err)
		assert.Equal(t, d.Name, r.Name)
		assert.Equal(t, requestID, r.RequestID)
		assert.Equal(t, scheduler.StatusRunning, r.Status)
	})

	t.Run("WriteStatusToExistingFile", func(t *testing.T) {
		d := &digraph.DAG{Name: "test_append_to_existing", Location: "test_append_to_existing.yaml"}
		requestID := "request-id-test-write-status-to-existing-file"

		dw, file, err := te.JSONDB.newWriter(d.Location, time.Now(), requestID)
		require.NoError(t, err)
		require.NoError(t, dw.open())

		initialStatus := model.NewStatus(d, nil, scheduler.StatusCancel, 10000, nil, nil)
		initialStatus.RequestID = requestID
		require.NoError(t, dw.write(initialStatus))
		require.NoError(t, dw.close())

		// Verify initial write
		data, err := te.JSONDB.FindByRequestID(d.Location, requestID)
		require.NoError(t, err)
		assert.Equal(t, scheduler.StatusCancel, data.Status.Status)
		assert.Equal(t, file, data.File)

		// Append to existing file
		dw = &writer{target: file}
		require.NoError(t, dw.open())
		initialStatus.Status = scheduler.StatusSuccess
		require.NoError(t, dw.write(initialStatus))
		require.NoError(t, dw.close())

		// Verify appended data
		data, err = te.JSONDB.FindByRequestID(d.Location, requestID)
		require.NoError(t, err)
		assert.Equal(t, scheduler.StatusSuccess, data.Status.Status)
		assert.Equal(t, file, data.File)
	})
}

func TestWriterErrorHandling(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	t.Run("OpenNonExistentDirectory", func(t *testing.T) {
		w := &writer{target: "/nonexistent/dir/file.dat"}
		assert.Error(t, w.open())
	})

	t.Run("WriteToClosedWriter", func(t *testing.T) {
		w := &writer{target: filepath.Join(te.TmpDir, "test.dat")}
		require.NoError(t, w.open())
		require.NoError(t, w.close())

		d := &digraph.DAG{Name: "test", Location: "test.yaml"}
		status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
		assert.Error(t, w.write(status))
	})

	t.Run("CloseMultipleTimes", func(t *testing.T) {
		w := &writer{target: filepath.Join(te.TmpDir, "test.dat")}
		require.NoError(t, w.open())
		require.NoError(t, w.close())
		assert.NoError(t, w.close()) // Second close should not return an error
	})
}

func TestWriterRename(t *testing.T) {
	te := setup(t)
	defer te.cleanup()

	oldName := "test_rename_old"
	newName := "test_rename_new"
	d := &digraph.DAG{
		Name:     oldName,
		Location: filepath.Join(te.TmpDir, oldName+".yaml"),
	}
	oldPath := d.Location
	newPath := filepath.Join(te.TmpDir, newName+".yaml")

	// Create a file
	dw, _, err := te.JSONDB.newWriter(d.Location, time.Now(), "request-id-1")
	require.NoError(t, err)
	require.NoError(t, dw.open())
	status := model.NewStatus(d, nil, scheduler.StatusRunning, 10000, nil, nil)
	require.NoError(t, dw.write(status))
	require.NoError(t, dw.close())

	oldDir := te.JSONDB.getDirectory(oldPath, oldName)
	newDir := te.JSONDB.getDirectory(newPath, newName)

	require.DirExists(t, oldDir)
	require.NoDirExists(t, newDir)

	err = te.JSONDB.Rename(oldPath, newPath)
	require.NoError(t, err)

	require.NoDirExists(t, oldDir)
	require.DirExists(t, newDir)

	ret := te.JSONDB.ReadStatusRecent(newPath, 1)
	require.Len(t, ret, 1)
	assert.Equal(t, status.RequestID, ret[0].Status.RequestID)
}
