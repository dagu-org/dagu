package cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/utils"
)

func TestMain(m *testing.M) {
	tmpDir := utils.MustTempDir("dagu_test")
	changeHomeDir(tmpDir)
	code := m.Run()
	os.RemoveAll(tmpDir)
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	os.Setenv("HOME", homeDir)
	_ = config.LoadConfig(homeDir)
}

func TestStartCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args:        []string{"start", testDAGFile("start.yaml")},
			expectedOut: []string{"1 finished"},
		},
		{
			args:        []string{"start", testDAGFile("start_with_params.yaml")},
			expectedOut: []string{"params is p1 and p2"},
		},
		{
			args:        []string{"start", `--params="p3 p4"`, testDAGFile("start_with_params.yaml")},
			expectedOut: []string{"params is p3 and p4"},
		},
	}

	for _, tc := range tests {
		testRunCommand(t, createStartCommand(), tc)
	}
}

func TestDryCommand(t *testing.T) {
	tests := []cmdTest{
		{
			args:        []string{"dry", testDAGFile("dry.yaml")},
			expectedOut: []string{"Starting DRY-RUN"},
		},
	}

	for _, tc := range tests {
		testRunCommand(t, createDryCommand(), tc)
	}
}

func TestRestartCommand(t *testing.T) {
	dagFile := testDAGFile("restart.yaml")

	// Start the DAG.
	go func() {
		testRunCommand(t, createStartCommand(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})
	}()

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG running.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Restart the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createRestartCommand(), cmdTest{args: []string{"restart", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG running again.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Stop the restarted DAG.
	testRunCommand(t, createStopCommand(), cmdTest{args: []string{"stop", dagFile}})

	time.Sleep(time.Millisecond * 100)

	// Wait for the DAG is stopped.
	testStatusEventual(t, dagFile, scheduler.SchedulerStatus_None)

	// Check parameter was the same as the first execution
	d, err := loadDAG(dagFile, "")
	require.NoError(t, err)
	ctrl := controller.NewDAGController(d)
	sts := ctrl.GetRecentStatuses(2)
	require.Len(t, sts, 2)
	require.Equal(t, sts[0].Status.Params, sts[1].Status.Params)

	<-done
}

func TestRetryCommand(t *testing.T) {
	dagFile := testDAGFile("retry.yaml")

	// Run a DAG.
	testRunCommand(t, createStartCommand(), cmdTest{args: []string{"start", `--params="foo"`, dagFile}})

	// Find the request ID.
	dsts, err := controller.NewDAGStatusReader().ReadStatus(dagFile, false)
	require.NoError(t, err)
	require.Equal(t, dsts.Status.Status, scheduler.SchedulerStatus_Success)
	require.NotNil(t, dsts.Status)

	reqID := dsts.Status.RequestId

	// Retry with the request ID.
	testRunCommand(t, createRetryCommand(), cmdTest{
		args:        []string{"retry", fmt.Sprintf("--req=%s", reqID), dagFile},
		expectedOut: []string{"param is foo"},
	})
}

func TestSchedulerCommand(t *testing.T) {
	// Start the scheduler.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createSchedulerCommand(), cmdTest{
			args:        []string{"scheduler"},
			expectedOut: []string{"starting dagu scheduler"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)

	// Stop the scheduler.
	signalChan <- syscall.SIGTERM
	<-done
}

func TestServerCommand(t *testing.T) {
	port := findPort(t)

	// Start the server.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createServerCommand(), cmdTest{
			args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			expectedOut: []string{"server is running"},
		})
		close(done)
	}()

	time.Sleep(time.Millisecond * 300)

	// Stop the server.
	res, err := http.Post(
		fmt.Sprintf("http://%s:%s/shutdown", "localhost", port),
		"application/json",
		nil,
	)

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode)

	<-done
}

func TestStatusCommand(t *testing.T) {
	dagFile := testDAGFile("status.yaml")

	// Start the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createStartCommand(), cmdTest{args: []string{"start", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 50)

	// Wait for the DAG running.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Check the current status.
	testRunCommand(t, createStatusCommand(), cmdTest{
		args:        []string{"status", dagFile},
		expectedOut: []string{"Status=running"},
	})

	// Stop the DAG.
	testRunCommand(t, createStopCommand(), cmdTest{args: []string{"stop", dagFile}})
	<-done
}

func TestStopCommand(t *testing.T) {
	dagFile := testDAGFile("stop.yaml")

	// Start the DAG.
	done := make(chan struct{})
	go func() {
		testRunCommand(t, createStartCommand(), cmdTest{args: []string{"start", dagFile}})
		close(done)
	}()

	time.Sleep(time.Millisecond * 50)

	// Wait for the DAG running.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Running)

	// Stop the DAG.
	testRunCommand(t, createStopCommand(), cmdTest{
		args:        []string{"stop", dagFile},
		expectedOut: []string{"Stopping..."}})

	// Check the last execution is cancelled.
	testLastStatusEventual(t, dagFile, scheduler.SchedulerStatus_Cancel)
	<-done
}

func TestVersionCommand(t *testing.T) {
	constants.Version = "1.2.3"
	testRunCommand(t, createVersionCommand(), cmdTest{
		args:        []string{"version"},
		expectedOut: []string{"1.2.3"}})
}

func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}
