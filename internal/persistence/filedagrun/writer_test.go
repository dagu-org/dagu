package filedagrun

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core/status"
	"github.com/dagu-org/dagu/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriter(t *testing.T) {
	th := setupTestStore(t)

	t.Run("WriteStatusToNewFile", func(t *testing.T) {
		dag := th.DAG("test_write_status")
		dagRunID := uuid.Must(uuid.NewV7()).String()
		dagRunStatus := models.NewStatusBuilder(dag.DAG).Create(dagRunID, status.Running, 1, time.Now())
		writer := dag.Writer(t, dagRunID, time.Now())
		writer.Write(t, dagRunStatus)

		writer.AssertContent(t, "test_write_status", dagRunID, status.Running)
	})

	t.Run("WriteStatusToExistingFile", func(t *testing.T) {
		dag := th.DAG("test_append_to_existing")
		dagRunID := uuid.Must(uuid.NewV7()).String()
		startedAt := time.Now()

		writer := dag.Writer(t, dagRunID, startedAt)

		dagRunStatus := models.NewStatusBuilder(dag.DAG).Create(dagRunID, status.Cancel, 1, time.Now())

		// Write initial status
		writer.Write(t, dagRunStatus)
		writer.Close(t)
		writer.AssertContent(t, "test_append_to_existing", dagRunID, status.Cancel)

		// Append to existing file
		dataRoot := NewDataRoot(th.TmpDir, dag.Name)
		run, err := dataRoot.FindByDAGRunID(th.Context, dagRunID)
		require.NoError(t, err)

		latestRun, err := run.LatestAttempt(th.Context, nil)
		require.NoError(t, err)

		err = latestRun.Open(th.Context)
		require.NoError(t, err)
		defer func() {
			_ = latestRun.Close(th.Context)
		}()

		// Append new status
		dagRunStatus.Status = status.Success
		err = latestRun.Write(th.Context, dagRunStatus)
		require.NoError(t, err)

		// Verify appended data
		writer.AssertContent(t, "test_append_to_existing", dagRunID, status.Success)
	})
}

func TestWriterErrorHandling(t *testing.T) {
	th := setupTestStore(t)

	t.Run("OpenNonExistentDirectory", func(t *testing.T) {
		writer := NewWriter("/nonexistent/dir/file.dat")
		err := writer.Open()
		assert.Error(t, err)
	})

	t.Run("WriteToClosedWriter", func(t *testing.T) {
		writer := NewWriter(filepath.Join(th.TmpDir, "test.dat"))
		require.NoError(t, writer.Open())
		require.NoError(t, writer.close())

		dag := th.DAG("test_write_to_closed_writer")
		dagRunID := uuid.Must(uuid.NewV7()).String()
		dagRunStatus := models.NewStatusBuilder(dag.DAG).Create(dagRunID, status.Running, 1, time.Now())
		assert.Error(t, writer.write(dagRunStatus))
	})

	t.Run("CloseMultipleTimes", func(t *testing.T) {
		writer := NewWriter(filepath.Join(th.TmpDir, "test.dat"))
		require.NoError(t, writer.Open())
		require.NoError(t, writer.close())
		assert.NoError(t, writer.close()) // Second close should not return an error
	})
}

func TestWriterRename(t *testing.T) {
	th := setupTestStore(t)

	// Create a status file with old path
	dag := th.DAG("test_rename_old")
	writer := dag.Writer(t, "dag-run-id-1", time.Now())
	dagRunID := uuid.Must(uuid.NewV7()).String()
	dagRunStatus := models.NewStatusBuilder(dag.DAG).Create(dagRunID, status.Running, 1, time.Now())
	writer.Write(t, dagRunStatus)
	writer.Close(t)
	require.FileExists(t, writer.FilePath)

	// Rename and verify the file
	newDAG := th.DAG("test_rename_new")
	err := th.Store.RenameDAGRuns(context.Background(), dag.Location, newDAG.Location)
	require.NoError(t, err)
	newWriter := newDAG.Writer(t, "dag-run-id-2", time.Now())

	require.NoFileExists(t, writer.FilePath)
	require.FileExists(t, newWriter.FilePath)
}
