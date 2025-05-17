package localproc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestProc(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	fileName := filepath.Join(th.Config.Paths.ProcDir, "test_proc")
	proc := NewProc(fileName)

	ctx := th.Context
	err := proc.Start(ctx)
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

func TestProc_Restart(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	fileName := filepath.Join(th.Config.Paths.ProcDir, "test_proc")
	proc := NewProc(fileName)

	ctx := th.Context
	err := proc.Start(ctx)
	require.NoError(t, err)

	// Restart the process
	err = proc.Stop(ctx)
	require.NoError(t, err)

	err = proc.Start(ctx)
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
