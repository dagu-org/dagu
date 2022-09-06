package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/scheduler"
)

func Test_restartCommand(t *testing.T) {
	cfg := testConfig("restart.yaml")
	cl := &dag.Loader{}
	d, _ := cl.Load(cfg, "")
	c := controller.New(d)

	// start the DAG
	println("start the DAG")
	go func() {
		app := makeApp()
		test := appTest{
			args: []string{"", "start", "--params=restart_test", cfg}, errored: false,
		}
		runAppTest(app, test, t)
	}()

	require.Eventually(t, func() bool {
		s, _ := c.GetStatus()
		return s.Status == scheduler.SchedulerStatus_Running
	}, time.Second*5, time.Millisecond*50)

	time.Sleep(time.Millisecond * 50)

	// restart the DAG
	go func() {
		app2 := makeApp()
		runAppTestOutput(app2, appTest{
			args: []string{"", "restart", cfg}, errored: false,
			output: []string{"wait for restart 1s", "Restarting"},
		}, t)
	}()

	// check canceled
	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		return s != nil && s.Status == scheduler.SchedulerStatus_Cancel
	}, time.Second*5, time.Millisecond*50)

	// check restarted
	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		return s != nil && s.Status == scheduler.SchedulerStatus_Running
	}, time.Second*5, time.Millisecond*50)

	// cancel the DAG
	go func() {
		app3 := makeApp()
		runAppTestOutput(app3, appTest{
			args: []string{"", "stop", cfg}, errored: false,
			output: []string{"Stopping..."},
		}, t)
	}()

	// check canceled
	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		return s != nil && s.Status == scheduler.SchedulerStatus_Cancel
	}, time.Second*5, time.Millisecond*50)

	// check history
	db := &database.Database{Config: database.DefaultConfig()}
	require.Eventually(t, func() bool {
		s := db.ReadStatusHist(cfg, 100)
		return len(s) == 2 && s[1].Status.Status == scheduler.SchedulerStatus_Cancel
	}, time.Second*5, time.Millisecond*50)

	// check result
	s := db.ReadStatusHist(cfg, 2)
	require.Equal(t, "restart_test", s[0].Status.Params)
	require.Equal(t, "restart_test", s[1].Status.Params)
}
