package localproc

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
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
	proc, err := procFiles.Acquire(ctx, digraph.DAGRunRef{
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
	count, err := procFiles.Count(ctx, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Wait for the process to stop
	<-done

	// Check if the count is 0
	count, err = procFiles.Count(ctx, name)
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
	count, err := procFiles.Count(ctx, name)
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
	proc, err := pg.Acquire(ctx, digraph.DAGRunRef{
		Name: "test_proc",
		ID:   "test_id",
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
	binary.BigEndian.PutUint64(buf, uint64(time.Now().Add(-pg.staleTime).Unix()))
	_, err = fd.WriteAt(buf, 0)

	// Close the file
	_ = fd.Sync()
	_ = fd.Close()

	// Check the count of alive proc files is still 1 because the file is not stale yet
	// due to the modification time
	count, err := pg.Count(ctx, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Update the file's modification time to be older than the stale time
	err = os.Chtimes(proc.fileName, time.Now().Add(-pg.staleTime), time.Now().Add(-pg.staleTime))
	require.NoError(t, err, "failed to update file times")

	// Check the count of alive proc files is 0 because the file is stale
	count, err = pg.Count(ctx, name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}
