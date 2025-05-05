package filestore

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/runstore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	th := setupTestJSONDB(t)

	t.Run("WriteStatusToNewFile", func(t *testing.T) {
		dag := th.DAG("test_write_status")
		requestID := uuid.Must(uuid.NewV7()).String()
		status := runstore.NewStatusBuilder(dag.DAG).Create(
			requestID, scheduler.StatusRunning, 1, time.Now(),
		)
		writer := dag.Writer(t, requestID, time.Now())
		writer.Write(t, status)

		writer.AssertContent(t, "test_write_status", requestID, scheduler.StatusRunning)
	})

	t.Run("WriteStatusToExistingFile", func(t *testing.T) {
		dag := th.DAG("test_append_to_existing")
		requestID := uuid.Must(uuid.NewV7()).String()
		startedAt := time.Now()

		writer := dag.Writer(t, requestID, startedAt)

		status := runstore.NewStatusBuilder(dag.DAG).Create(
			requestID, scheduler.StatusCancel, 1, time.Now(),
		)

		// Write initial status
		writer.Write(t, status)
		writer.Close(t)
		writer.AssertContent(t, "test_append_to_existing", requestID, scheduler.StatusCancel)

		// Append to existing file
		dataRoot := NewDataRoot(th.tmpDir, dag.Name)
		run, err := dataRoot.FindByRequestID(th.Context, requestID)
		require.NoError(t, err)

		record, err := run.LatestRecord(th.Context, nil)
		require.NoError(t, err)

		err = record.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = record.Close(th.Context)
		}()

		// Append new status
		status.Status = scheduler.StatusSuccess
		err = record.Write(th.Context, status)
		require.NoError(t, err)

		// Verify appended data
		writer.AssertContent(t, "test_append_to_existing", requestID, scheduler.StatusSuccess)
	})
}

func TestWriterErrorHandling(t *testing.T) {
	th := setupTestJSONDB(t)

	t.Run("OpenNonExistentDirectory", func(t *testing.T) {
		writer := NewWriter("/nonexistent/dir/file.dat")
		err := writer.Open()
		assert.Error(t, err)
	})

	t.Run("WriteToClosedWriter", func(t *testing.T) {
		writer := NewWriter(filepath.Join(th.tmpDir, "test.dat"))
		require.NoError(t, writer.Open())
		require.NoError(t, writer.close())

		dag := th.DAG("test_write_to_closed_writer")
		requestID := uuid.Must(uuid.NewV7()).String()
		status := runstore.NewStatusBuilder(dag.DAG).Create(requestID, scheduler.StatusRunning, 1, time.Now())
		assert.Error(t, writer.write(status))
	})

	t.Run("CloseMultipleTimes", func(t *testing.T) {
		writer := NewWriter(filepath.Join(th.tmpDir, "test.dat"))
		require.NoError(t, writer.Open())
		require.NoError(t, writer.close())
		assert.NoError(t, writer.close()) // Second close should not return an error
	})
}

func TestWriterRename(t *testing.T) {
	th := setupTestJSONDB(t)

	// Create a status file with old path
	dag := th.DAG("test_rename_old")
	writer := dag.Writer(t, "request-id-1", time.Now())
	requestID := uuid.Must(uuid.NewV7()).String()
	status := runstore.NewStatusBuilder(dag.DAG).Create(requestID, scheduler.StatusRunning, 1, time.Now())
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
