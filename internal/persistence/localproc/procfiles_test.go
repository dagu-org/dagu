package localproc

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
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
