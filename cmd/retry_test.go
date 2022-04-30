package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/yohamta/dagman/internal/controller"
	"github.com/yohamta/dagman/internal/database"
	"github.com/yohamta/dagman/internal/scheduler"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_retryCommand(t *testing.T) {
	app := makeApp()
	configPath := testConfig("cmd_retry.yaml")
	runAppTestOutput(app, appTest{
		args: []string{"", "start", "--params=x", configPath}, errored: true,
		output: []string{},
	}, t)

	dag, err := controller.FromConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, dag.Status.Status, scheduler.SchedulerStatus_Error)

	db := database.New(database.DefaultConfig())
	status, err := db.FindByRequestId(configPath, dag.Status.RequestId)
	require.NoError(t, err)
	dw, err := db.NewWriterFor(configPath, status.File)
	require.NoError(t, err)
	err = dw.Open()
	require.NoError(t, err)

	for _, n := range status.Status.Nodes {
		n.Command = "true"
	}
	err = dw.Write(status.Status)
	require.NoError(t, err)

	time.Sleep(time.Second)

	app = makeApp()
	runAppTestOutput(app, appTest{
		args: []string{"", "retry", fmt.Sprintf("--req=%s",
			dag.Status.RequestId), testConfig("cmd_retry.yaml")}, errored: false,
		output: []string{"parameter is x"},
	}, t)

	assert.Eventually(t, func() bool {
		dag, err = controller.FromConfig(testConfig("cmd_retry.yaml"))
		if err != nil {
			return false
		}
		return dag.Status.Status == scheduler.SchedulerStatus_Success
	}, time.Millisecond*3000, time.Millisecond*100)

	dag, err = controller.FromConfig(testConfig("cmd_retry.yaml"))
	require.NoError(t, err)
	require.NotEqual(t, status.Status.RequestId, dag.Status.RequestId)
}
