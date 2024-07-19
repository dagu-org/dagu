package agent_test

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/test"
	"github.com/google/uuid"

	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/dag/scheduler"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

func TestAgent_Run(t *testing.T) {
	t.Run("RunDAG", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "run.yaml")
		eng := setup.Engine()
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		latestStatus, err := eng.GetLatestStatus(workflow)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, latestStatus.Status)

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		time.Sleep(100 * time.Millisecond)

		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusSuccess
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("DeleteOldHistory", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Create a history file by running a DAG
		workflow := testLoadDAG(t, "simple.yaml")
		eng := setup.Engine()
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		err := dagAgent.Run(context.Background())
		require.NoError(t, err)
		history := eng.GetRecentHistory(workflow, 2)
		require.Equal(t, 1, len(history))

		// Set the retention days to 0 and run the DAG again
		workflow.HistRetentionDays = 0
		dagAgent = newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		err = dagAgent.Run(context.Background())
		require.NoError(t, err)

		// Check if only the latest history file exists
		history = eng.GetRecentHistory(workflow, 2)
		require.Equal(t, 1, len(history))
	})
	t.Run("AlreadyRunning", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "is_running.yaml")
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		go func() {
			_ = dagAgent.Run(context.Background())
		}()

		time.Sleep(time.Millisecond * 30)

		curStatus := dagAgent.Status()
		require.NotNil(t, curStatus)
		require.Equal(t, curStatus.Status, scheduler.StatusRunning)

		dagAgent = newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		err := dagAgent.Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "is already running")
	})
	t.Run("PreConditionNotMet", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "multiple_steps.yaml")

		// Precondition is not met
		workflow.Preconditions = []dag.Condition{{Condition: "`echo 1`", Expected: "0"}}

		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		err := dagAgent.Run(context.Background())
		require.Error(t, err)

		// Check if all nodes are not executed
		status := dagAgent.Status()
		require.Equal(t, scheduler.StatusCancel, status.Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[0].Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[1].Status)
	})
	t.Run("FinishWithError", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Run a DAG that fails
		errDAG := testLoadDAG(t, "error.yaml")
		dagAgent := newAgent(setup, newReqID(), errDAG, &agent.AgentOpts{})
		err := dagAgent.Run(context.Background())
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status().Status)
	})
	t.Run("ReceiveSignal", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		abortFunc := func(a *agent.Agent) { a.Signal(syscall.SIGTERM) }

		workflow := testLoadDAG(t, "sleep.yaml")
		eng := setup.Engine()
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		go func() {
			_ = dagAgent.Run(context.Background())
		}()

		// wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*1, time.Millisecond*100)

		// send a signal to cancel the DAG
		abortFunc(dagAgent)

		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*1, time.Millisecond*100)
	})
	t.Run("ExitHandler", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		workflow := testLoadDAG(t, "on_exit.yaml")
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		err := dagAgent.Run(context.Background())
		require.NoError(t, err)

		// Check if the DAG is executed successfully
		status := dagAgent.Status()
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
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{
			Dry: true,
		})

		err := dagAgent.Run(context.Background())
		require.NoError(t, err)

		curStatus := dagAgent.Status()
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		eng := setup.Engine()
		history := eng.GetRecentHistory(workflow, 1)
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

		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		err := dagAgent.Run(context.Background())
		require.Error(t, err)

		// Check if the DAG failed
		status := dagAgent.Status()
		require.Equal(t, scheduler.StatusError, status.Status)

		// Modify the DAG to make it successful
		for _, node := range status.Nodes {
			node.CmdWithArgs = "true"
		}

		// Retry the DAG and check if it is successful
		dagAgent = newAgent(setup, newReqID(), workflow, &agent.AgentOpts{
			RetryTarget: status,
		})
		err = dagAgent.Run(context.Background())
		require.NoError(t, err)

		status = dagAgent.Status()
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
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})
		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		eng := setup.Engine()
		require.Eventually(t, func() bool {
			status, _ := eng.GetLatestStatus(workflow)
			// require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		status, err := model.StatusFromJSON(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		dagAgent.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)

	})
	t.Run("HTTP_InvalidRequest", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Start a long-running DAG
		workflow := testLoadDAG(t, "handle_http2.yaml")
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		eng := setup.Engine()
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		var mockResponseWriter = mockResponseWriter{}

		// Request with an invalid path
		dagAgent.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "GET",
			URL:    &url.URL{Path: "/invalid-path"},
		})
		require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

		// Stop the DAG
		dagAgent.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("HTTP_HandleCancel", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		// Start a long-running DAG
		workflow := testLoadDAG(t, "handle_http3.yaml")
		dagAgent := newAgent(setup, newReqID(), workflow, &agent.AgentOpts{})

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		eng := setup.Engine()
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Cancel the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "POST",
			URL:    &url.URL{Path: "/stop"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)
		require.Equal(t, "OK", mockResponseWriter.body)

		// Wait for the DAG to stop
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(workflow)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)
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

func newReqID() string {
	id, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return id.String()
}

func newAgent(
	setup test.Setup,
	reqID string,
	workflow *dag.DAG,
	opts *agent.AgentOpts,
) *agent.Agent {
	return agent.New(
		reqID,
		workflow,
		test.NewLogger(),
		setup.Config.LogDir,
		"",
		setup.Engine(),
		setup.DataStore(),
		opts,
	)
}
