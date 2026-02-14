package cmd_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStartAllCommand(t *testing.T) {
	t.Run("StartAll", func(t *testing.T) {
		th := test.SetupCommand(t, test.WithCoordinatorEnabled())
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				fmt.Sprintf("--port=%s", findPort(t)),
				"--coordinator.host=0.0.0.0",
				fmt.Sprintf("--coordinator.port=%s", findPort(t)),
			},
			ExpectedOut: []string{"Server initialization", "Scheduler initialization", "Coordinator initialization"},
		})

	})
	t.Run("StartAllWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				"--config", test.TestdataPath(t, "cli/config_startall.yaml"),
				fmt.Sprintf("--coordinator.port=%s", findPort(t)),
			},
			ExpectedOut: []string{"54322", "dagu_test", "Coordinator initialization"},
		})
	})
}
