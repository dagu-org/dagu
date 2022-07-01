package main

import (
	"fmt"
	"syscall"
	"testing"
	"time"
)

func Test_schedulerCommand(t *testing.T) {
	app := makeApp()
	dir := testHomeDir

	done := make(chan struct{})
	go func() {
		runAppTestOutput(app, appTest{
			args: []string{"", "scheduler",
				fmt.Sprintf("--dags=%s", dir)}, errored: false,
			output: []string{"starting dagu scheduler"},
		}, t)
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)

	sigs <- syscall.SIGTERM
	<-done
}
