// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package agent_test

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "run.yaml")
		cli := setup.Client()
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		latestStatus, err := cli.GetLatestStatus(workflow)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, latestStatus.Status)

		go func() {
			err := agt.Run(context.Background())
			require.NoError(t, err)
		}()

		time.Sleep(100 * time.Millisecond)

		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusSuccess
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Create a history file by running a DAG
		workflow := testLoadDAG(t, "simple.yaml")
		cli := setup.Client()
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		err := agt.Run(context.Background())
		require.NoError(t, err)
		history := cli.GetRecentHistory(workflow, 2)
		require.Equal(t, 1, len(history))

		// Set the retention days to 0 and run the DAG again
		workflow.HistRetentionDays = 0
		agt = newAgent(setup, genRequestID(), workflow, &agent.Options{})
		err = agt.Run(context.Background())
		require.NoError(t, err)

		// Check if only the latest history file exists
		history = cli.GetRecentHistory(workflow, 2)
		require.Equal(t, 1, len(history))
	})
	t.Run("AlreadyRunning", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "is_running.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		go func() {
			_ = agt.Run(context.Background())
		}()

		time.Sleep(time.Millisecond * 30)

		curStatus := agt.Status()
		require.NotNil(t, curStatus)
		require.Equal(t, curStatus.Status, scheduler.StatusRunning)

		agt = newAgent(setup, genRequestID(), workflow, &agent.Options{})
		err := agt.Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "is already running")
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "multiple_steps.yaml")

		// Precondition is not met
		workflow.Preconditions = []dag.Condition{{Condition: "`echo 1`", Expected: "0"}}

		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})
		err := agt.Run(context.Background())
		require.Error(t, err)

		// Check if all nodes are not executed
		status := agt.Status()
		require.Equal(t, scheduler.StatusCancel, status.Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[0].Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[1].Status)
	})
	t.Run("FinishWithError", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Run a DAG that fails
		errDAG := testLoadDAG(t, "error.yaml")
		agt := newAgent(setup, genRequestID(), errDAG, &agent.Options{})
		err := agt.Run(context.Background())
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, agt.Status().Status)
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Run a DAG that timeout
		timeoutDAG := testLoadDAG(t, "timeout.yaml")
		agt := newAgent(setup, genRequestID(), timeoutDAG, &agent.Options{})
		ctx := context.Background()
		err := agt.Run(ctx)
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, agt.Status().Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		abortFunc := func(a *agent.Agent) { a.Signal(syscall.SIGTERM) }

		workflow := testLoadDAG(t, "sleep.yaml")
		cli := setup.Client()
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		go func() {
			_ = agt.Run(context.Background())
		}()

		// wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*1, time.Millisecond*100)

		// send a signal to cancel the DAG
		abortFunc(agt)

		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*1, time.Millisecond*100)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "on_exit.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})
		err := agt.Run(context.Background())
		require.NoError(t, err)

		// Check if the DAG is executed successfully
		status := agt.Status()
		require.Equal(t, scheduler.StatusSuccess, status.Status)
		for _, s := range status.Nodes {
			require.Equal(t, scheduler.NodeStatusSuccess, s.Status)
		}

		// Check if the exit handler is executed
		require.Equal(t, scheduler.NodeStatusSuccess, status.OnExit.Status)
	})
}

func TestAgent_DryRun(t *testing.T) {
	t.Parallel()
	t.Run("DryRun", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "dry.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{
			Dry: true,
		})

		err := agt.Run(context.Background())
		require.NoError(t, err)

		curStatus := agt.Status()
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		cli := setup.Client()
		history := cli.GetRecentHistory(workflow, 1)
		require.Equal(t, 0, len(history))
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Parallel()
	t.Run("RetryDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// retry.yaml has a DAG that fails
		workflow := testLoadDAG(t, "retry.yaml")

		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})
		err := agt.Run(context.Background())
		require.Error(t, err)

		// Check if the DAG failed
		status := agt.Status()
		require.Equal(t, scheduler.StatusError, status.Status)

		// Modify the DAG to make it successful
		for _, node := range status.Nodes {
			node.Step.CmdWithArgs = "true"
		}

		// Retry the DAG and check if it is successful
		agt = newAgent(setup, genRequestID(), workflow, &agent.Options{
			RetryTarget: status,
		})
		err = agt.Run(context.Background())
		require.NoError(t, err)

		status = agt.Status()
		require.Equal(t, scheduler.StatusSuccess, status.Status)

		for _, node := range status.Nodes {
			if node.Status != scheduler.NodeStatusSuccess &&
				node.Status != scheduler.NodeStatusSkipped {
				t.Errorf("invalid status: %s", node.Status.String())
			}
		}
	})
}

func TestAgent_HandleHTTP(t *testing.T) {
	t.Parallel()
	t.Run("HTTP_Valid", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Start a long-running DAG
		workflow := testLoadDAG(t, "handle_http.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})
		go func() {
			err := agt.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := setup.Client()
		require.Eventually(t, func() bool {
			status, _ := cli.GetLatestStatus(workflow)
			// require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		agt.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		status, err := model.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		agt.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)

	})
	t.Run("HTTP_InvalidRequest", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Start a long-running DAG
		workflow := testLoadDAG(t, "handle_http2.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		go func() {
			err := agt.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := setup.Client()
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		var mockResponseWriter = mockResponseWriter{}

		// Request with an invalid path
		agt.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/invalid-path"},
		})
		require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

		// Stop the DAG
		agt.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("HTTP_HandleCancel", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Start a long-running DAG
		workflow := testLoadDAG(t, "handle_http3.yaml")
		agt := newAgent(setup, genRequestID(), workflow, &agent.Options{})

		go func() {
			err := agt.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := setup.Client()
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Cancel the DAG
		var mockResponseWriter = mockResponseWriter{}
		agt.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/stop"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)
		require.Equal(t, "OK", mockResponseWriter.body)

		// Wait for the DAG to stop
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*3, time.Millisecond*100)
	})
}

// Assert that mockResponseWriter implements http.ResponseWriter
var _ http.ResponseWriter = (*mockResponseWriter)(nil)

type mockResponseWriter struct {
	status int
	body   string
	header *http.Header
}

func (h *mockResponseWriter) Header() http.Header {
	if h.header == nil {
		h.header = &http.Header{}
	}
	return *h.header
}

func (h *mockResponseWriter) Write(body []byte) (int, error) {
	h.body = string(body)
	return len([]byte(h.body)), nil
}

func (h *mockResponseWriter) WriteHeader(statusCode int) {
	h.status = statusCode
}

// testLoadDAG load the specified DAG file for testing
// without base config or parameters.
func testLoadDAG(t *testing.T, name string) *dag.DAG {
	file := filepath.Join(util.MustGetwd(), "testdata", name)
	workflow, err := dag.Load("", file, "")
	require.NoError(t, err)
	return workflow
}

func genRequestID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func newAgent(
	setup test.Setup,
	requestID string,
	workflow *dag.DAG,
	opts *agent.Options,
) *agent.Agent {
	return agent.New(
		requestID,
		workflow,
		test.NewLogger(),
		setup.Config.LogDir,
		"",
		setup.Config.DAGQueueLength,
		setup.Client(),
		setup.DataStore(),
		setup.DataStore().QueueStore(),
		setup.DataStore().StatsStore(),
		opts,
	)
}
