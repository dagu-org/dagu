package cmd

import (
	"testing"
	"time"
)

func TestSchedulerCommand(t *testing.T) {
	// Start the scheduler.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createSchedulerCommand(), cmdTest{
			args:        []string{"scheduler"},
			expectedOut: []string{"starting dagu scheduler"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)
}
