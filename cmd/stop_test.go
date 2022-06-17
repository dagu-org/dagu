package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/scheduler"
)

func Test_stopCommand(t *testing.T) {
	c := testConfig("cmd_stop_sleep.yaml")
	test := appTest{
		args: []string{"", "start", c}, errored: false,
	}

	app := makeApp()
	stopper := makeApp()

	go func() {
		time.Sleep(time.Millisecond * 50)
		runAppTestOutput(stopper, appTest{
			args: []string{"", "stop", test.args[2]}, errored: false,
			output: []string{"Stopping..."},
		}, t)
	}()

	runAppTest(app, test, t)

	db := database.New(database.DefaultConfig())
	cfg := &config.Config{
		ConfigPath: c,
	}

	s := db.ReadStatusHist(cfg.ConfigPath, 1)
	require.Equal(t, 1, len(s))
	require.Equal(t, scheduler.SchedulerStatus_Cancel, s[0].Status.Status)
}
