package main

import (
	"fmt"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

func Test_schedulerCommand(t *testing.T) {
	app := makeApp()
	dir := utils.MustTempDir("dagu_test_scheduler")
	originalHome := os.ExpandEnv("${HOME}")
	os.Setenv("HOME", dir)
	settings.ChangeHomeDir(dir)
	defer func() {
		os.Setenv("HOME", originalHome)
		settings.ChangeHomeDir(originalHome)
	}()

	cfg := fmt.Sprintf(`dags: "%s"`, dir)
	err := os.MkdirAll(path.Join(dir, ".dagu"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(path.Join(dir, ".dagu/admin.yaml"), []byte(cfg), 0644)
	require.NoError(t, err)

	dag := `schedule: "* * * * *"
steps:
  - name: "test"
    command: "true" 
`
	dagFile := path.Join(dir, "dag.yml")
	err = os.WriteFile(dagFile, []byte(dag), 0644)
	require.NoError(t, err)

	done := make(chan struct{})
	go func() {
		runAppTestOutput(app, appTest{
			args: []string{"", "scheduler"}, errored: false,
			output: []string{"starting dagu scheduler"},
		}, t)
		close(done)
	}()

	time.Sleep(time.Millisecond * 500)

	sigs <- syscall.SIGTERM

	cl := &config.Loader{}
	d, err := cl.Load(dagFile, "")
	require.NoError(t, err)

	c := controller.New(d)

	require.Eventually(t, func() bool {
		s, _ := c.GetLastStatus()
		println(fmt.Sprintf("%+v", s))
		return s.Status == scheduler.SchedulerStatus_Success
	}, time.Second*2, time.Millisecond*100)

	<-done
}
