package cmd

import (
	"testing"
	"time"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		setup := setupTest(t)
		defer setup.cleanup()

		go func() {
			testRunCommand(t, schedulerCmd(), cmdTest{
				args:        []string{"scheduler"},
				expectedOut: []string{"starting dagu scheduler"},
			})
		}()

		time.Sleep(time.Millisecond * 500)
	})
}
