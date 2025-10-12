package fileproc

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcGroup(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	name := "test_proc"
	procFiles := NewProcGroup(baseDir, name, time.Hour)

	// Create a proc file
	proc, err := procFiles.Acquire(ctx, core.DAGRunRef{
		Name: "test_proc",
		ID:   "test_id",
	})
	require.NoError(t, err, "failed to get proc")

	// Start the process
	err = proc.startHeartbeat(ctx)
	require.NoError(t, err, "failed to start proc")

	// Stop the process after a short delay
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 100) // Give some time for the file to be created
		err := proc.Stop(ctx)
		require.NoError(t, err, "failed to stop proc")
		close(done)
	}()

	// Check if the count is 1
	count, err := procFiles.Count(ctx)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Wait for the process to stop
	<-done

	// Check if the count is 0
	count, err = procFiles.Count(ctx)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestProcGroup_Empty(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	name := "test_proc"
	procFiles := NewProcGroup(baseDir, name, time.Hour)

	// Check if the count is 0
	count, err := procFiles.Count(ctx)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestProcGroup_IsStale(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	name := "test_proc"
	pg := NewProcGroup(baseDir, name, time.Second*5)

	// create a proc
	proc, err := pg.Acquire(ctx, core.DAGRunRef{
		Name: "test_proc",
		ID:   "test_id",
	})
	require.NoError(t, err, "failed to get proc")

	// Make sure the directory exists
	err = os.MkdirAll(filepath.Dir(proc.fileName), 0750)
	require.NoError(t, err, "failed to create proc directory")

	// Create the proc file
	fd, err := os.OpenFile(proc.fileName, os.O_CREATE|os.O_RDWR, 0600)
	assert.NoError(t, err, "failed to create proc file")

	// Write a timestamp that is older than the stale time
	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Add(-pg.staleTime).Unix()))
	_, err = fd.WriteAt(buf, 0)
	require.NoError(t, err, "failed to write timestamp to proc file")

	// Close the file
	_ = fd.Sync()
	_ = fd.Close()

	// Check the count of alive proc files is still 1 because the file is not stale yet
	// due to the modification time
	count, err := pg.Count(ctx)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Update the file's modification time to be older than the stale time
	err = os.Chtimes(proc.fileName, time.Now().Add(-pg.staleTime), time.Now().Add(-pg.staleTime))
	require.NoError(t, err, "failed to update file times")

	// Check the count of alive proc files is 0 because the file is stale
	count, err = pg.Count(ctx)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}

func TestProcGroup_IsRunAlive(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	baseDir := t.TempDir()
	name := "test_proc"
	pg := NewProcGroup(baseDir, name, time.Second*5)

	t.Run("NoProcessFile", func(t *testing.T) {
		dagRun := core.DAGRunRef{
			Name: name,
			ID:   "run-123",
		}

		// Test when no process file exists
		alive, err := pg.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("AliveProcess", func(t *testing.T) {
		dagRun := core.DAGRunRef{
			Name: name,
			ID:   "run-456",
		}

		// Create a process
		proc, err := pg.Acquire(ctx, dagRun)
		require.NoError(t, err)

		// Start heartbeat
		err = proc.startHeartbeat(ctx)
		require.NoError(t, err)

		// Give a moment for the heartbeat to write
		time.Sleep(time.Millisecond * 50)

		// Check if the run is alive
		alive, err := pg.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.True(t, alive)

		// Stop the process
		err = proc.Stop(ctx)
		require.NoError(t, err)

		// Check again - should be false now
		alive, err = pg.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.False(t, alive)
	})

	t.Run("DifferentRunID", func(t *testing.T) {
		// Create a process for one run ID
		dagRun1 := core.DAGRunRef{
			Name: name,
			ID:   "run-789",
		}
		proc1, err := pg.Acquire(ctx, dagRun1)
		require.NoError(t, err)

		// Start heartbeat
		err = proc1.startHeartbeat(ctx)
		require.NoError(t, err)

		// Give a moment for the heartbeat to write
		time.Sleep(time.Millisecond * 50)

		// Check for a different run ID
		dagRun2 := core.DAGRunRef{
			Name: name,
			ID:   "run-999",
		}
		alive, err := pg.IsRunAlive(ctx, dagRun2)
		require.NoError(t, err)
		require.False(t, alive)

		// Check the original run is still alive
		alive, err = pg.IsRunAlive(ctx, dagRun1)
		require.NoError(t, err)
		require.True(t, alive)

		// Cleanup
		err = proc1.Stop(ctx)
		require.NoError(t, err)
	})

	t.Run("StaleProcess", func(t *testing.T) {
		// Create a ProcGroup with very short stale time
		shortPG := NewProcGroup(baseDir, name, time.Millisecond*100)

		dagRun := core.DAGRunRef{
			Name: name,
			ID:   "run-stale",
		}

		// Create a process
		proc, err := shortPG.Acquire(ctx, dagRun)
		require.NoError(t, err)

		// Ensure directory exists
		err = os.MkdirAll(filepath.Dir(proc.fileName), 0750)
		require.NoError(t, err)

		// Create the proc file with old timestamp
		fd, err := os.OpenFile(proc.fileName, os.O_CREATE|os.O_RDWR, 0600)
		require.NoError(t, err)

		// Write an old timestamp
		buf := make([]byte, 8)
		oldTime := time.Now().Add(-time.Second * 10)
		binary.BigEndian.PutUint64(buf, uint64(oldTime.Unix()))
		_, err = fd.WriteAt(buf, 0)
		require.NoError(t, err)
		_ = fd.Close()

		// Set old modification time
		err = os.Chtimes(proc.fileName, oldTime, oldTime)
		require.NoError(t, err)

		// Check if the run is alive (should be false and file should be cleaned up)
		alive, err := shortPG.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.False(t, alive)

		// Verify file was removed
		_, err = os.Stat(proc.fileName)
		require.True(t, os.IsNotExist(err))
	})

	t.Run("StaleProcessRemovesEmptyDir", func(t *testing.T) {
		// Create a unique subdirectory for this test
		testSubDir := filepath.Join(baseDir, "stale-test-subdir")
		err := os.MkdirAll(testSubDir, 0750)
		require.NoError(t, err)

		// Create a ProcGroup with very short stale time
		shortPG := NewProcGroup(testSubDir, name, time.Millisecond*100)

		dagRun := core.DAGRunRef{
			Name: name,
			ID:   "run-stale-dir",
		}

		// Create a process in a subdirectory
		proc, err := shortPG.Acquire(ctx, dagRun)
		require.NoError(t, err)

		// The proc file will be in a subdirectory like testSubDir/YYYY-MM-DD/
		procDir := filepath.Dir(proc.fileName)

		// Ensure directory exists
		err = os.MkdirAll(procDir, 0750)
		require.NoError(t, err)

		// Create the proc file with old timestamp
		fd, err := os.OpenFile(proc.fileName, os.O_CREATE|os.O_RDWR, 0600)
		require.NoError(t, err)

		// Write an old timestamp
		buf := make([]byte, 8)
		oldTime := time.Now().Add(-time.Second * 10)
		binary.BigEndian.PutUint64(buf, uint64(oldTime.Unix()))
		_, err = fd.WriteAt(buf, 0)
		require.NoError(t, err)
		_ = fd.Close()

		// Set old modification time
		err = os.Chtimes(proc.fileName, oldTime, oldTime)
		require.NoError(t, err)

		// Check if the run is alive (should be false and file should be cleaned up)
		alive, err := shortPG.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.False(t, alive)

		// Verify file was removed
		_, err = os.Stat(proc.fileName)
		require.True(t, os.IsNotExist(err))

		// Verify the parent directory was also removed (if it's empty)
		_, err = os.Stat(procDir)
		require.True(t, os.IsNotExist(err), "empty parent directory should be removed")
	})

	t.Run("InvalidFilePattern", func(t *testing.T) {
		dagRun := core.DAGRunRef{
			Name: name,
			ID:   "run-invalid",
		}

		// Create directory
		err := os.MkdirAll(baseDir, 0750)
		require.NoError(t, err)

		// Create a file with invalid pattern but matching run ID
		invalidFile := filepath.Join(baseDir, "invalid_file_run-invalid.proc")
		err = os.WriteFile(invalidFile, []byte("test"), 0600)
		require.NoError(t, err)

		// Should return false as the file doesn't match the regex pattern
		alive, err := pg.IsRunAlive(ctx, dagRun)
		require.NoError(t, err)
		require.False(t, alive)

		// Clean up
		_ = os.Remove(invalidFile)
	})
}
