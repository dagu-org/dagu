package main

import (
	"testing"
	"time"
)

func Test_statusCommand(t *testing.T) {
	tests := []appTest{
		{
			args: []string{"", "start", testConfig("cmd_status.yaml")}, errored: false,
		},
	}

	for _, v := range tests {
		app := makeApp()
		app2 := makeApp()

		done := make(chan bool)
		go func() {
			time.Sleep(time.Millisecond * 50)
			runAppTestOutput(app2, appTest{
				args: []string{"", "status", v.args[2]}, errored: false,
				output: []string{"Status=running"},
			}, t)
			done <- true
		}()

		runAppTest(app, v, t)
		<-done
	}
}
