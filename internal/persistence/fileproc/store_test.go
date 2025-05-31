package fileproc

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/stretchr/testify/require"
)

func TestStore(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	baseDir := t.TempDir()
	store := New(baseDir)

	// Create a dagRun reference
	dagRun := digraph.DAGRunRef{
		Name: "test_dag",
		ID:   "test_id",
	}

	// Get the process for the dag-run
	proc, err := store.Acquire(ctx, dagRun)
	require.NoError(t, err, "failed to get proc")

	// Stop the process after a short delay
	done := make(chan struct{})
	go func() {
		time.Sleep(time.Millisecond * 100) // Give some time for the file to be created
		err := proc.Stop(ctx)
		require.NoError(t, err, "failed to stop proc")
		close(done)
	}()

	// Check if the count is 1
	count, err := store.CountAlive(ctx, dagRun.Name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 1, count, "expected 1 proc file")

	// Wait for the process to stop
	<-done

	// Check if the count is 0
	count, err = store.CountAlive(ctx, dagRun.Name)
	require.NoError(t, err, "failed to count proc files")
	require.Equal(t, 0, count, "expected 0 proc files")
}
