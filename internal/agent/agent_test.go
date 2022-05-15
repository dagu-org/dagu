package agent

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var testsDir = path.Join(utils.MustGetwd(), "../../tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("agent_test")
	settings.InitTest(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestRunDAG(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_run.yaml"))
	require.NoError(t, err)

	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
}

func TestCheckRunning(t *testing.T) {
	config := testConfig("agent_is_running.yaml")
	dag, err := controller.FromConfig(config)
	require.NoError(t, err)

	a := &Agent{Config: &Config{
		DAG: dag.Config,
	}}

	go func() {
		a.Run()
	}()

	time.Sleep(time.Millisecond * 30)

	status := a.Status()
	require.NotNil(t, status)
	require.Equal(t, status.Status, scheduler.SchedulerStatus_Running)

	_, err = testDAG(t, dag)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is already running")
}

func TestDryRun(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_dry.yaml"))
	require.NoError(t, err)

	a := &Agent{Config: &Config{
		DAG: dag.Config,
		Dry: true,
	}}
	err = a.Run()
	require.NoError(t, err)

	status := a.Status()
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
}

func TestCancelDAG(t *testing.T) {
	for _, abort := range []func(*Agent){
		func(a *Agent) { a.Signal(syscall.SIGTERM) },
		func(a *Agent) { a.Cancel() },
	} {
		a, dag := testDAGAsync(t, testConfig("agent_sleep.yaml"))
		time.Sleep(time.Millisecond * 100)
		abort(a)
		time.Sleep(time.Millisecond * 500)
		status, err := controller.New(dag.Config).GetLastStatus()
		require.NoError(t, err)
		assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	}
}

func TestPreConditionInvalid(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_multiple_steps.yaml"))
	require.NoError(t, err)

	dag.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "0",
		},
	}

	status, err := testDAG(t, dag)
	require.Error(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	assert.Equal(t, scheduler.NodeStatus_None, status.Nodes[0].Status)
	assert.Equal(t, scheduler.NodeStatus_None, status.Nodes[1].Status)
}

func TestPreConditionValid(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_with_params.yaml"))
	require.NoError(t, err)

	dag.Config.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "1",
		},
	}
	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
}

func TestOnExit(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_on_exit.yaml"))
	require.NoError(t, err)
	status, err := testDAG(t, dag)
	require.NoError(t, err)

	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		assert.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
	assert.Equal(t, scheduler.NodeStatus_Success, status.OnExit.Status)
}

func TestRetry(t *testing.T) {
	cfg := testConfig("agent_retry.yaml")
	dag, err := controller.FromConfig(cfg)
	require.NoError(t, err)

	status, err := testDAG(t, dag)
	require.Error(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_Error, status.Status)

	for _, n := range status.Nodes {
		n.Command = "true"
	}
	a := &Agent{
		Config: &Config{
			DAG: dag.Config,
		},
		RetryConfig: &RetryConfig{
			Status: status,
		},
	}
	err = a.Run()
	status = a.Status()
	require.NoError(t, err)
	assert.Equal(t, scheduler.SchedulerStatus_Success, status.Status)

	for _, n := range status.Nodes {
		if n.Status != scheduler.NodeStatus_Success &&
			n.Status != scheduler.NodeStatus_Skipped {
			t.Errorf("invalid status: %s", n.Status.String())
		}
	}
}

func TestHandleHTTP(t *testing.T) {
	dag, err := controller.FromConfig(testConfig("agent_handle_http.yaml"))
	require.NoError(t, err)

	a := &Agent{Config: &Config{
		DAG: dag.Config,
	}}

	go func() {
		err := a.Run()
		require.Error(t, err)
	}()

	<-time.After(time.Millisecond * 50)

	var mockResponseWriter = mockResponseWriter{}

	// status
	r := &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/status",
		},
	}

	a.handleHTTP(&mockResponseWriter, r)
	require.Equal(t, http.StatusOK, mockResponseWriter.status)

	status, err := models.StatusFromJson(mockResponseWriter.body)
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Running, status.Status)

	// invalid path
	r = &http.Request{
		Method: "GET",
		URL: &url.URL{
			Path: "/invalid-path",
		},
	}
	a.handleHTTP(&mockResponseWriter, r)
	require.Equal(t, http.StatusNotFound, mockResponseWriter.status)

	// cancel
	r = &http.Request{
		Method: "POST",
		URL: &url.URL{
			Path: "/stop",
		},
	}
	a.handleHTTP(&mockResponseWriter, r)
	require.Equal(t, http.StatusOK, mockResponseWriter.status)
	require.Equal(t, "OK", mockResponseWriter.body)

	<-time.After(time.Millisecond * 50)

	status = a.Status()
	require.Equal(t, status.Status, scheduler.SchedulerStatus_Cancel)
}

type mockResponseWriter struct {
	status int
	body   string
	header *http.Header
}

var _ (http.ResponseWriter) = (*mockResponseWriter)(nil)

func (h *mockResponseWriter) Header() http.Header {
	if h.header == nil {
		h.header = &http.Header{}
	}
	return *h.header
}

func (h *mockResponseWriter) Write(body []byte) (int, error) {
	h.body = string(body)
	return 0, nil
}

func (h *mockResponseWriter) WriteHeader(statusCode int) {
	h.status = statusCode
}

func testDAG(t *testing.T, dag *controller.DAG) (*models.Status, error) {
	t.Helper()
	a := &Agent{Config: &Config{
		DAG: dag.Config,
	}}
	err := a.Run()
	return a.Status(), err
}

func testConfig(name string) string {
	return path.Join(testsDir, name)
}

func testDAGAsync(t *testing.T, file string) (*Agent, *controller.DAG) {
	t.Helper()

	dag, err := controller.FromConfig(file)
	require.NoError(t, err)

	a := &Agent{Config: &Config{
		DAG: dag.Config,
	}}

	go func() {
		a.Run()
	}()

	return a, dag
}
