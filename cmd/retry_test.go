package cmd

import (
	"fmt"
	"testing"
	"time"

	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"

	"github.com/stretchr/testify/require"
)

func Test_retryCommand(t *testing.T) {
	cmd := startCmd
	configPath := testConfig("cmd_retry.yaml")
	runCmdTestOutput(cmd, cmdTest{
		args: []string{configPath}, errored: true,
		flags:  map[string]string{"params": "x"},
		output: []string{},
	}, t)

	dag, err := controller.FromConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, dag.Status.Status, scheduler.SchedulerStatus_Success)

	db := database.New(database.DefaultConfig())
	status, err := db.FindByRequestId(configPath, dag.Status.RequestId)
	require.NoError(t, err)
	status.Status.Nodes[0].Status = scheduler.NodeStatus_Error
	status.Status.Status = scheduler.SchedulerStatus_Error

	w := &database.Writer{Target: status.File}
	require.NoError(t, w.Open())
	require.NoError(t, w.Write(status.Status))
	require.NoError(t, w.Close())

	time.Sleep(time.Millisecond * 1000)

	cmd2 := retryCmd
	runCmdTestOutput(cmd2, cmdTest{
		args:    []string{testConfig("cmd_retry.yaml")},
		flags:   map[string]string{"req": fmt.Sprintf("%s", dag.Status.RequestId)},
		errored: false,
		output:  []string{"parameter is x"},
	}, t)

	c := controller.New(dag.Config)

	var retryStatus *models.Status
	require.Eventually(t, func() bool {
		retryStatus, err = c.GetLastStatus()
		if err != nil {
			return false
		}
		return retryStatus.Status == scheduler.SchedulerStatus_Success
	}, time.Millisecond*3000, time.Millisecond*100)

	require.NoError(t, err)
	require.NotEqual(t, retryStatus.RequestId, dag.Status.RequestId)
}

func Test_retryFail(t *testing.T) {
	configPath := testConfig("cmd_retry.yaml")
	require.Error(t, retry(configPath, "invalid-request-id"))
}
