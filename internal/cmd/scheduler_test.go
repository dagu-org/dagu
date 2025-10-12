package cmd_test

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, cmd.Scheduler(), test.CmdTest{
			Args:        []string{"scheduler"},
			ExpectedOut: []string{"Scheduler started"},
		})
	})
	t.Run("StartSchedulerWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, cmd.Scheduler(), test.CmdTest{
			Args:        []string{"scheduler", "--config", test.TestdataPath(t, "cli/config_test.yaml")},
			ExpectedOut: []string{"dagu_test"},
		})
	})
}
