package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		th := test.SetupCommandTest(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, schedulerCmd(), test.CmdTest{
			Args:        []string{"scheduler"},
			ExpectedOut: []string{"Scheduler started"},
		})
	})
	t.Run("StartSchedulerWithConfig", func(t *testing.T) {
		th := test.SetupCommandTest(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, schedulerCmd(), test.CmdTest{
			Args:        []string{"scheduler", "--config", test.TestdataPath(t, "cmd/config_test.yaml")},
			ExpectedOut: []string{"dagu_test"},
		})
	})
}
