package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, schedulerCmd(), cmdTest{
			args:        []string{"scheduler"},
			expectedOut: []string{"Scheduler started"},
		})
	})
	t.Run("StartSchedulerWithConfig", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()

		th.RunCommand(t, schedulerCmd(), cmdTest{
			args:        []string{"scheduler", "--config", test.TestdataPath(t, "cmd/config_test.yaml")},
			expectedOut: []string{"/.dagu_test/config/dags"},
		})
	})
}
