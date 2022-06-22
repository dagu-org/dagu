package cmd

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
	test := cmdTest{
		args: []string{c}, errored: false,
	}

	cmd := startCmd
	stopper := stopCmd

	go func() {
		time.Sleep(time.Millisecond * 50)
		runCmdTestOutput(stopper, cmdTest{
			args: []string{test.args[0]}, errored: false,
			output: []string{"Stopping..."},
		}, t)
	}()

	runCmdTest(cmd, test, t)

	db := database.New(database.DefaultConfig())
	cfg := &config.Config{
		ConfigPath: c,
	}

	s := db.ReadStatusHist(cfg.ConfigPath, 1)
	require.Equal(t, 1, len(s))
	require.Equal(t, scheduler.SchedulerStatus_Cancel, s[0].Status.Status)
}
