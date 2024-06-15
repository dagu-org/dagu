package agent_test

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/persistence"
	"github.com/dagu-dev/dagu/internal/persistence/client"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/dagu-dev/dagu/internal/persistence/model"
	"github.com/dagu-dev/dagu/internal/scheduler"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

// setupTest sets temporary directories and loads the configuration.
func setupTest(t *testing.T) (string, engine.Engine, persistence.DataStoreFactory) {
	t.Helper()

	tmpDir := util.MustTempDir("dagu_test")
	err := os.Setenv("HOME", tmpDir)
	require.NoError(t, err)

	err = config.LoadConfig()
	require.NoError(t, err)

	dataStore := client.NewDataStoreFactory(&config.Config{
		DataDir: path.Join(tmpDir, ".dagu", "data"),
	})

	return tmpDir, engine.New(dataStore, new(engine.Config), config.Get()), dataStore
}

func TestAgent_Run(t *testing.T) {
	t.Run("Run a DAG successfully", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dg := testLoadDAG(t, "run.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)

		latestStatus, err := eng.GetLatestStatus(dg)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusNone, latestStatus.Status)

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		time.Sleep(100 * time.Millisecond)

		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
			require.NoError(t, err)
			return status.Status == scheduler.StatusSuccess
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("Old history files are deleted", func(t *testing.T) {
		_, eng, dataStore := setupTest(t)

		// Create a history file by running a DAG
		dg := testLoadDAG(t, "run.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)
		err := dagAgent.Run(context.Background())
		require.NoError(t, err)
		history := eng.GetRecentHistory(dg, 2)
		require.Equal(t, 1, len(history))

		// Set the retention days to 0 and run the DAG again
		dg.HistRetentionDays = 0
		dagAgent = agent.New(&agent.Config{DAG: dg}, eng, dataStore)
		err = dagAgent.Run(context.Background())
		require.NoError(t, err)

		// Check if only the latest history file exists
		history = eng.GetRecentHistory(dg, 2)
		require.Equal(t, 1, len(history))
	})
	t.Run("It should not run a DAG if it is already running", func(t *testing.T) {
		tmpDir, eng, df := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dg := testLoadDAG(t, "is_running.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, df)

		go func() {
			_ = dagAgent.Run(context.Background())
		}()

		time.Sleep(time.Millisecond * 30)

		curStatus := dagAgent.Status()
		require.NotNil(t, curStatus)
		require.Equal(t, curStatus.Status, scheduler.StatusRunning)

		dagAgent = agent.New(&agent.Config{DAG: dg}, eng, df)
		err := dagAgent.Run(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "is already running")
	})
	t.Run("It should not run a DAG if the precondition is not met", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dg := testLoadDAG(t, "multiple_steps.yaml")

		// Precondition is not met
		dg.Preconditions = []*dag.Condition{{Condition: "`echo 1`", Expected: "0"}}

		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)
		err := dagAgent.Run(context.Background())
		require.Error(t, err)

		// Check if all nodes are not executed
		status := dagAgent.Status()
		require.Equal(t, scheduler.StatusCancel, status.Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[0].Status)
		require.Equal(t, scheduler.NodeStatusNone, status.Nodes[1].Status)
	})
	t.Run("Run a DAG and finish with an error", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// Run a DAG that fails
		dagAgent := agent.New(
			&agent.Config{DAG: testLoadDAG(t, "error.yaml")},
			eng,
			dataStore,
		)
		err := dagAgent.Run(context.Background())
		require.Error(t, err)

		// Check if the status is saved correctly
		require.Equal(t, scheduler.StatusError, dagAgent.Status().Status)
	})
	t.Run("Run a DAG and receive a signal", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		abortFunc := func(a *agent.Agent) { a.Signal(syscall.SIGTERM) }

		dg := testLoadDAG(t, "sleep.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)

		go func() {
			_ = dagAgent.Run(context.Background())
		}()

		// wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*1, time.Millisecond*100)

		// send a signal to cancel the DAG
		abortFunc(dagAgent)

		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*1, time.Millisecond*100)
	})
	t.Run("Run a DAG and execute the exit handler", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dg := testLoadDAG(t, "on_exit.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)
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
	t.Run("Dry-run a DAG successfully", func(t *testing.T) {
		tmpDir, eng, df := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		dg := testLoadDAG(t, "dry.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg, Dry: true}, eng, df)

		err := dagAgent.Run(context.Background())
		require.NoError(t, err)

		curStatus := dagAgent.Status()
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusSuccess, curStatus.Status)

		// Check if the status is not saved
		history := eng.GetRecentHistory(dg, 1)
		require.Equal(t, 0, len(history))
	})
}

func TestAgent_Retry(t *testing.T) {
	t.Run("Retry a DAG", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// retry.yaml has a DAG that fails
		dg := testLoadDAG(t, "retry.yaml")

		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)
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
		dagAgent = agent.New(&agent.Config{DAG: dg, RetryTarget: status}, eng, dataStore)
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
	t.Run("Handle HTTP requests and return the status of the DAG", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// Start a long-running DAG
		dg := testLoadDAG(t, "handle_http.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)
		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		time.Sleep(time.Second * 2)

		// Wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
			log.Println(status.Status.String())
			if err != nil {
				log.Panicln(err.Error())
			}
			require.NoError(t, err)
			return status.Status == scheduler.StatusRunning
		}, time.Second*2, time.Millisecond*100)

		// Get the status of the DAG
		var mockResponseWriter = mockResponseWriter{}
		dagAgent.HandleHTTP(&mockResponseWriter, &http.Request{
			Method: "GET", URL: &url.URL{Path: "/status"},
		})
		require.Equal(t, http.StatusOK, mockResponseWriter.status)

		// Check if the status is returned correctly
		status, err := model.StatusFromJson(mockResponseWriter.body)
		require.NoError(t, err)
		require.Equal(t, scheduler.StatusRunning, status.Status)

		// Stop the DAG
		dagAgent.Signal(syscall.SIGTERM)
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)

	})
	t.Run("Handle invalid HTTP requests", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// Start a long-running DAG
		dg := testLoadDAG(t, "handle_http2.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
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
			status, err := eng.GetLatestStatus(dg)
			require.NoError(t, err)
			return status.Status == scheduler.StatusCancel
		}, time.Second*2, time.Millisecond*100)
	})
	t.Run("Handle cancel request and stop the DAG", func(t *testing.T) {
		tmpDir, eng, dataStore := setupTest(t)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		// Start a long-running DAG
		dg := testLoadDAG(t, "handle_http3.yaml")
		dagAgent := agent.New(&agent.Config{DAG: dg}, eng, dataStore)

		go func() {
			err := dagAgent.Run(context.Background())
			require.NoError(t, err)
		}()

		// Wait for the DAG to start
		require.Eventually(t, func() bool {
			status, err := eng.GetLatestStatus(dg)
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
			status, err := eng.GetLatestStatus(dg)
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
	file := path.Join(util.MustGetwd(), "testdata", name)
	dg, err := dag.Load("", file, "")
	require.NoError(t, err)
	return dg
}
