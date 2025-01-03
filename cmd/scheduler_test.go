package main

import (
	"testing"
	"time"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			th.RunCommand(t, schedulerCmd(), cmdTest{
				args:        []string{"scheduler"},
				expectedOut: []string{"starting dagu scheduler"},
			})
		}()

		// Wait for the scheduler to start.
		time.Sleep(time.Millisecond * 500)
	})
}
