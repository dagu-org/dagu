// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package jsondb

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	th := testSetup(t)

	t.Run("WriteStatusToNewFile", func(t *testing.T) {
		dag := th.DAG("test_write_status")
		status := model.NewStatus(dag.DAG, nil, scheduler.StatusRunning, 10000, nil, nil)
		requestID := fmt.Sprintf("request-id-%d", time.Now().Unix())
		status.RequestID = requestID

		writer := dag.Writer(t, requestID, time.Now())
		writer.Write(t, status)

		writer.AssertContent(t, "test_write_status", requestID, scheduler.StatusRunning)
	})

	t.Run("WriteStatusToExistingFile", func(t *testing.T) {
		dag := th.DAG("test_append_to_existing")
		requestID := "request-id-test-write-status-to-existing-file"
		startedAt := time.Now()

		writer := dag.Writer(t, requestID, startedAt)

		status := model.NewStatus(dag.DAG, nil, scheduler.StatusCancel, 10000, nil, nil)
		status.RequestID = requestID

		// Write initial status
		writer.Write(t, status)
		writer.Close(t)
		writer.AssertContent(t, "test_append_to_existing", requestID, scheduler.StatusCancel)

		// Append to existing file
		writer = dag.Writer(t, requestID, startedAt)
		status.Status = scheduler.StatusSuccess
		writer.Write(t, status)
		writer.Close(t)

		// Verify appended data
		writer.AssertContent(t, "test_append_to_existing", requestID, scheduler.StatusSuccess)
	})
}

func TestWriterErrorHandling(t *testing.T) {
	th := testSetup(t)

	t.Run("OpenNonExistentDirectory", func(t *testing.T) {
		writer := newWriter("/nonexistent/dir/file.dat")
		err := writer.open()
		assert.Error(t, err)
	})

	t.Run("WriteToClosedWriter", func(t *testing.T) {
		writer := newWriter(filepath.Join(th.tmpDir, "test.dat"))
		require.NoError(t, writer.open())
		require.NoError(t, writer.close())

		dag := th.DAG("test_write_to_closed_writer")
		status := model.NewStatus(dag.DAG, nil, scheduler.StatusRunning, 10000, nil, nil)
		assert.Error(t, writer.write(status))
	})

	t.Run("CloseMultipleTimes", func(t *testing.T) {
		writer := newWriter(filepath.Join(th.tmpDir, "test.dat"))
		require.NoError(t, writer.open())
		require.NoError(t, writer.close())
		assert.NoError(t, writer.close()) // Second close should not return an error
	})
}

func TestWriterRename(t *testing.T) {
	th := testSetup(t)

	// Create a status file with old path
	dag := th.DAG("test_rename_old")
	writer := dag.Writer(t, "request-id-1", time.Now())
	status := model.NewStatus(dag.DAG, nil, scheduler.StatusRunning, 10000, nil, nil)
	writer.Write(t, status)
	writer.Close(t)
	require.FileExists(t, writer.FilePath)

	// Rename and verify the file
	newDAG := th.DAG("test_rename_new")
	err := th.DB.Rename(context.Background(), dag.Location, newDAG.Location)
	require.NoError(t, err)
	newWriter := newDAG.Writer(t, "request-id-2", time.Now())

	require.NoFileExists(t, writer.FilePath)
	require.FileExists(t, newWriter.FilePath)
}
