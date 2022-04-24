package main

import (
	"jobctl/internal/config"
	"jobctl/internal/database"
	"jobctl/internal/scheduler"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_stopCommand(t *testing.T) {
	c := testConfig("cmd_stop_sleep.yaml")
	test := appTest{
		args: []string{"", "start", c}, errored: false,
	}

	app := makeApp()
	stopper := makeApp()
	done := make(chan bool)

	go func() {
		time.Sleep(time.Millisecond * 50)
		runAppTestOutput(stopper, appTest{
			args: []string{"", "stop", test.args[2]}, errored: false,
			output: []string{"stopped"},
		}, t)
		done <- true
	}()

	runAppTest(app, test, t)

	<-done

	db := database.New(database.DefaultConfig())
	cfg := &config.Config{
		ConfigPath: c,
	}
	s, err := db.ReadStatusHist(cfg.ConfigPath, 1)
	require.NoError(t, err)
	require.Equal(t, 1, len(s))
	assert.Equal(t, scheduler.SchedulerStatus_Cancel, s[0].Status.Status)
}
