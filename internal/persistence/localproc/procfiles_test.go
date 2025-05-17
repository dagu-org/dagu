package localproc

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcFiles(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	baseDir := th.Config.Paths.ProcDir
	name := "test_proc"
	procFiles := NewProcFiles(baseDir, name)

	// Create a proc file
	proc, err := procFiles.GetProc(th.Context, digraph.WorkflowRef{
		Name:       "test_proc",
		WorkflowID: "test_id",
	})
	require.NoError(t, err, "failed to get proc")

	// Start the process
	err = proc.Start(th.Context)
	require.NoError(t, err, "failed to start proc")

	// Stop the process after a short delay
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 100) // Give some time for the file to be created
		err = proc.Stop(th.Context)
		require.NoError(t, err, "failed to stop proc")
		close(done)
	}()

	// Check if the count is 1
	count, err := procFiles.Count(th.Context, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Wait for the process to stop
	<-done

	// Check if the count is 0
	count, err = procFiles.Count(th.Context, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestProcFiles_Empty(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	baseDir := th.Config.Paths.ProcDir
	name := "test_proc"
	procFiles := NewProcFiles(baseDir, name)

	// Check if the count is 0
	count, err := procFiles.Count(th.Context, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestProcFiles_Stale(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	baseDir := th.Config.Paths.ProcDir
	name := "test_proc"
	procFiles := NewProcFiles(baseDir, name)

	// create a proc
	proc, err := procFiles.GetProc(th.Context, digraph.WorkflowRef{
		Name:       "test_proc",
		WorkflowID: "test_id",
	})
	require.NoError(t, err, "failed to get proc")

	// Make sure the directory exists
	err = os.MkdirAll(filepath.Dir(proc.fileName), 0755)
	require.NoError(t, err, "failed to create proc directory")

	// Create the proc file
	fd, err := os.OpenFile(proc.fileName, os.O_CREATE|os.O_RDWR, 0600)
	assert.NoError(t, err, "failed to create proc file")

	// Write a timestamp that is older than the stale time
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Add(-procFiles.staleTime).UnixNano()))
	_, err = fd.WriteAt(buf, 0)

	// Close the file
	_ = fd.Sync()
	_ = fd.Close()

	// Check the count of alive proc files is still 1 because the file is not stale yet
	// due to the modification time
	count, err := procFiles.Count(th.Context, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Update the file's modification time to be older than the stale time
	err = os.Chtimes(proc.fileName, time.Now().Add(-procFiles.staleTime), time.Now().Add(-procFiles.staleTime))
	require.NoError(t, err, "failed to update file times")

	// Check the count of alive proc files is 0 because the file is stale
	count, err = procFiles.Count(th.Context, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}
