package cli_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/apps/cli"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStartAllCommand(t *testing.T) {
	t.Run("StartAll", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cli.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				fmt.Sprintf("--port=%s", findPort(t)),
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
		th.RunCommand(t, cli.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				"--config", test.TestdataPath(t, "cli/config_startall.yaml"),
				fmt.Sprintf("--coordinator.port=%s", findPort(t)),
			},
			ExpectedOut: []string{"54322", "dagu_test", "Coordinator initialization"},
		})
	})
}
