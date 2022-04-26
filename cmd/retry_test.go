package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/yohamta/jobctl/internal/controller"
	"github.com/yohamta/jobctl/internal/database"
	"github.com/yohamta/jobctl/internal/scheduler"

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

	job, err := controller.FromConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, job.Status.Status, scheduler.SchedulerStatus_Error)

	db := database.New(database.DefaultConfig())
	status, err := db.FindByRequestId(configPath, job.Status.RequestId)
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
			job.Status.RequestId), testConfig("cmd_retry.yaml")}, errored: false,
		output: []string{"parameter is x"},
	}, t)

	assert.Eventually(t, func() bool {
		job, err = controller.FromConfig(testConfig("cmd_retry.yaml"))
		if err != nil {
			return false
		}
		return job.Status.Status == scheduler.SchedulerStatus_Success
	}, time.Millisecond*3000, time.Millisecond*100)

	job, err = controller.FromConfig(testConfig("cmd_retry.yaml"))
	require.NoError(t, err)
	require.NotEqual(t, status.Status.RequestId, job.Status.RequestId)
}
