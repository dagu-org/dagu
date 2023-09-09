package cmd

import (
	"os"
	"testing"
	"time"
)

func TestSchedulerCommand(t *testing.T) {
	tmpDir, _, _ := setupTest(t)
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	go func() {
		testRunCommand(t, createSchedulerCommand(), cmdTest{
			args:        []string{"scheduler"},
			expectedOut: []string{"starting dagu scheduler"},
		})
	}()

	time.Sleep(time.Millisecond * 500)
}
