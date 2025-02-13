package main

import (
	"testing"
	"time"
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
}
