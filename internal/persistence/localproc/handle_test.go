package localproc

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/models"
	"github.com/stretchr/testify/require"
)

func TestProcHandle(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	fileName := filepath.Join(tmpDir, "test_proc")
	proc := NewProcHandler(fileName, models.ProcMeta{})

	ctx := context.Background()
	err := proc.startHeartbeat(ctx)
	require.NoError(t, err)

	done := make(chan struct{})

	go func() {
		err := proc.Stop(ctx)
		require.NoError(t, err)
		close(done)
	}()

	// Check if the file is created
	_, err = os.Stat(fileName)
	require.NoError(t, err)

	<-done

	// Check if the file is deleted
	_, err = os.Stat(fileName)
	require.Error(t, err, "file should be deleted")
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestProcHandle_Restart(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()

	fileName := filepath.Join(tmpDir, "test_proc")
	proc := NewProcHandler(fileName, models.ProcMeta{})

	err := proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// Restart the process
	err = proc.Stop(ctx)
	require.NoError(t, err)

	err = proc.startHeartbeat(ctx)
	require.NoError(t, err)

	// Check if the file is created again
	_, err = os.Stat(fileName)
	require.NoError(t, err)

	err = proc.Stop(ctx)
	require.NoError(t, err)

	// Check if the file is deleted again
	_, err = os.Stat(fileName)
	require.Error(t, err, "file should be deleted")
	require.ErrorIs(t, err, os.ErrNotExist)
}
