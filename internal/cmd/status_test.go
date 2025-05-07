package cmd_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStatusCommand(t *testing.T) {
	t.Run("StatusDAG", func(t *testing.T) {
		th := test.SetupCommand(t)

		dagFile := th.DAG(t, "cmd/status.yaml")

		done := make(chan struct{})
		go func() {
			// Start a DAG to check the status.
			args := []string{"start", dagFile.Location}
			th.RunCommand(t, cmd.CmdStart(), test.CmdTest{Args: args})
			close(done)
		}()

		require.Eventually(t, func() bool {
			runRecords := th.HistoryRepo.Recent(th.Context, dagFile.Location, 1)
			if len(runRecords) < 1 {
				return false
			}
			status, err := runRecords[0].ReadStatus(th.Context)
			if err != nil {
				return false
			}

			return scheduler.StatusRunning == status.Status
		}, time.Second*3, time.Millisecond*50)

		// Check the current status.
		th.RunCommand(t, cmd.CmdStatus(), test.CmdTest{
			Args:        []string{"status", dagFile.Location},
			ExpectedOut: []string{"status=running"},
		})

		// Stop the DAG.
		args := []string{"stop", dagFile.Location}
		th.RunCommand(t, cmd.CmdStop(), test.CmdTest{Args: args})
		<-done
	})
}
