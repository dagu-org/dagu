// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

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
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		th := test.Setup(t)

		dag := testLoadDAG(t, "run.yaml")
		cli := th.Client()
		ctx := th.Context
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})

		latestStatus, err := cli.GetLatestStatus(ctx, dag)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, latestStatus.Status)

		go func() {
			err := agt.Run(ctx)
			require.NoError(t, err)
		}()

		time.Sleep(100 * time.Millisecond)

		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusSuccess
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		th := test.Setup(t)

		// Create a history file by running a DAG
		dag := testLoadDAG(t, "simple.yaml")
		cli := th.Client()
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context

		err := agt.Run(ctx)
		require.NoError(t, err)
		history := cli.GetRecentHistory(ctx, dag, 2)
		require.Equal(t, 1, len(history))

		// Set the retention days to 0 and run the DAG again
		dag.HistRetentionDays = 0
		agt = newAgent(th, genRequestID(), dag, &agent.Options{})
		err = agt.Run(ctx)
		require.NoError(t, err)

		// Check if only the latest history file exists
		history = cli.GetRecentHistory(ctx, dag, 2)
		require.Equal(t, 1, len(history))
	})
	t.Run("AlreadyRunning", func(t *testing.T) {
		th := test.Setup(t)

		dag := testLoadDAG(t, "is_running.yaml")
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context

		go func() {
			_ = agt.Run(ctx)
		}()

		time.Sleep(time.Millisecond * 30)

		curStatus := agt.Status()
		require.NotNil(t, curStatus)
		require.Equal(t, curStatus.Status, scheduler.StatusRunning)

		agt = newAgent(th, genRequestID(), dag, &agent.Options{})
		err := agt.Run(ctx)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is already running")
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		th := test.Setup(t)

		dag := testLoadDAG(t, "multiple_steps.yaml")

		// Precondition is not met
		dag.Preconditions = []digraph.Condition{{Condition: "`echo 1`", Expected: "0"}}

		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context
		err := agt.Run(ctx)
		require.Error(t, err)

		// Check if all nodes are not executed
		status := agt.Status()
		require.Equal(t, scheduler.StatusCancel, status.Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[0].Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[1].Status)
	})
	t.Run("FinishWithError", func(t *testing.T) {
		th := test.Setup(t)

		// Run a DAG that fails
		errDAG := testLoadDAG(t, "error.yaml")
		agt := newAgent(th, genRequestID(), errDAG, &agent.Options{})
		ctx := th.Context
		err := agt.Run(ctx)
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, agt.Status().Status)
	})
	t.Run("FinishWithTimeout", func(t *testing.T) {
		th := test.Setup(t)

		// Run a DAG that timeout
		timeoutDAG := testLoadDAG(t, "timeout.yaml")
		agt := newAgent(th, genRequestID(), timeoutDAG, &agent.Options{})
		ctx := th.Context
		err := agt.Run(ctx)
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, agt.Status().Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		th := test.Setup(t)

		ctx := th.Context

		abortFunc := func(a *agent.Agent) { a.Signal(ctx, syscall.SIGTERM) }

		dag := testLoadDAG(t, "sleep.yaml")
		cli := th.Client()
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})

		go func() {
			_ = agt.Run(ctx)
		}()

		// wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*1, time.Millisecond*100)

		// send a signal to cancel the DAG
		abortFunc(agt)

		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*1, time.Millisecond*100)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		th := test.Setup(t)

		dag := testLoadDAG(t, "on_exit.yaml")
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context
		err := agt.Run(ctx)
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
		th := test.Setup(t)

		dag := testLoadDAG(t, "dry.yaml")
		ctx := th.Context
		agt := newAgent(th, genRequestID(), dag, &agent.Options{
			Dry: true,
		})

		err := agt.Run(ctx)
		require.NoError(t, err)

		curStatus := agt.Status()
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		cli := th.Client()
		history := cli.GetRecentHistory(ctx, dag, 1)
		require.Equal(t, 0, len(history))
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Parallel()
	t.Run("RetryDAG", func(t *testing.T) {
		th := test.Setup(t)

		// retry.yaml has a DAG that fails
		dag := testLoadDAG(t, "retry.yaml")

		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context

		err := agt.Run(ctx)
		require.Error(t, err)

		// Check if the DAG failed
		status := agt.Status()
		require.Equal(t, scheduler.StatusError, status.Status)

		// Modify the DAG to make it successful
		for _, node := range status.Nodes {
			node.Step.CmdWithArgs = "true"
		}

		// Retry the DAG and check if it is successful
		agt = newAgent(th, genRequestID(), dag, &agent.Options{
			RetryTarget: status,
		})
		err = agt.Run(ctx)
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
		th := test.Setup(t)

		// Start a long-running DAG
		dag := testLoadDAG(t, "handle_http.yaml")
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context
		go func() {
			err := agt.Run(ctx)
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := th.Client()
		require.Eventually(t, func() bool {
			status, _ := cli.GetLatestStatus(ctx, dag)
			// require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		agt.HandleHTTP(ctx)(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		status, err := model.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		agt.Signal(ctx, syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)

	})
	t.Run("HTTP_InvalidRequest", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := testLoadDAG(t, "handle_http2.yaml")
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context

		go func() {
			err := agt.Run(ctx)
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := th.Client()
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		var mockResponseWriter = mockResponseWriter{}

		// Request with an invalid path
		agt.HandleHTTP(ctx)(&mockResponseWriter, &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/invalid-path"},
		})
		require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

		// Stop the DAG
		agt.Signal(ctx, syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("HTTP_HandleCancel", func(t *testing.T) {
		th := test.Setup(t)

		// Start a long-running DAG
		dag := testLoadDAG(t, "handle_http3.yaml")
		agt := newAgent(th, genRequestID(), dag, &agent.Options{})
		ctx := th.Context

		go func() {
			err := agt.Run(ctx)
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		cli := th.Client()
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Cancel the DAG
		var mockResponseWriter = mockResponseWriter{}
		agt.HandleHTTP(ctx)(&mockResponseWriter, &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/stop"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)
		require.Equal(t, "OK", mockResponseWriter.body)

		// Wait for the DAG to stop
		require.Eventually(t, func() bool {
			status, err := cli.GetLatestStatus(ctx, dag)
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
func testLoadDAG(t *testing.T, name string) *digraph.DAG {
	file := filepath.Join(fileutil.MustGetwd(), "testdata", name)
	dag, err := digraph.Load(context.Background(), "", file, "")
	require.NoError(t, err)
	return dag
}

func genRequestID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func newAgent(
	th test.Helper,
	requestID string,
	dag *digraph.DAG,
	opts *agent.Options,
) *agent.Agent {
	logDir, logFile := th.Config.LogDir, ""
	return agent.New(
		requestID,
		dag,
		logDir,
		logFile,
		th.Client(),
		th.DataStore(),
		opts,
	)
}
