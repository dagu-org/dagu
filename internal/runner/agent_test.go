package runner

import (
	"os"
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func TestAgent(t *testing.T) {
	tmpDir := utils.MustTempDir("runner_agent_test")
	defer func() {
		os.RemoveAll(tmpDir)
	}()

	now := time.Date(2020, 1, 1, 1, 0, 0, 0, time.UTC)
	a := NewAgent(
		&admin.Config{
			DAGs:    path.Join(testsDir, "runner"),
			Command: testBin,
			LogDir:  path.Join(tmpDir, "log"),
		})
	utils.FixedTime = now

	go func() {
		err := a.Start()
		require.NoError(t, err)
	}()

	f := path.Join(testsDir, "runner/scheduled_job.yaml")
	cl := &config.Loader{}
	dag, err := cl.LoadHeadOnly(f)
	require.NoError(t, err)
	c := controller.New(dag)

	require.Eventually(t, func() bool {
		s, err := c.GetLastStatus()
		return err == nil && s.Status == scheduler.SchedulerStatus_Success
	}, time.Second*1, time.Millisecond*100)

	a.Stop()
}
