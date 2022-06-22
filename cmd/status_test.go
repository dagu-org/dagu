package cmd

import (
	"testing"
	"time"
)

func Test_statusCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args: []string{testConfig("cmd_status.yaml")}, errored: false,
		},
	}

	for _, v := range tests {
		cmd := startCmd
		cmd2 := statusCmd

		done := make(chan bool)
		go func() {
			time.Sleep(time.Millisecond * 50)
			runCmdTestOutput(cmd2, cmdTest{
				args: []string{v.args[0]}, errored: false,
				output: []string{"Status=running"},
			}, t)
			done <- true
		}()

		runCmdTest(cmd, v, t)
		<-done
	}
}
