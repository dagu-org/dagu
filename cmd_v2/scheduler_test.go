package cmd_v2

import (
	"syscall"
	"testing"
	"time"
)

func TestSchedulerCommand(t *testing.T) {
	// Start the scheduler.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, schedulerCommand(), cmdTest{
			args:        []string{"scheduler"},
			expectedOut: []string{"starting dagu scheduler"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)

	// Stop the scheduler.
	sigs <- syscall.SIGTERM
	<-done
}
