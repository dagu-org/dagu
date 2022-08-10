package dagu

import (
	"net/http"
	"net/url"
	"os"
	"path"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var testsDir = path.Join(utils.MustGetwd(), "./tests/testdata")

func TestMain(m *testing.M) {
	tempDir := utils.MustTempDir("agent_test")
	settings.ChangeHomeDir(tempDir)
	code := m.Run()
	os.RemoveAll(tempDir)
	os.Exit(code)
}

func TestRunDAG(t *testing.T) {
	cfg := testLoadDAG(t, "agent_run.yaml")

	a := &Agent{AgentConfig: &AgentConfig{DAG: cfg}}

	status, _ := controller.New(cfg).GetLastStatus()
	require.Equal(t, scheduler.SchedulerStatus_None, status.Status)

	go func() {
		err := a.Run()
		require.NoError(t, err)
	}()

	time.Sleep(100 * time.Millisecond)

	require.Eventually(t, func() bool {
		status, err := controller.New(cfg).GetLastStatus()
		require.NoError(t, err)
		return status.Status == scheduler.SchedulerStatus_Success
	}, time.Second*2, time.Millisecond*100)

	// check deletion of expired history files
	cfg.HistRetentionDays = 0
	a = &Agent{AgentConfig: &AgentConfig{DAG: cfg}}
	err := a.Run()
	require.NoError(t, err)
	statusList := controller.New(cfg).GetStatusHist(100)
	require.Equal(t, 1, len(statusList))
}

func TestCheckRunning(t *testing.T) {
	cfg := testLoadDAG(t, "agent_is_running.yaml")

	a := &Agent{AgentConfig: &AgentConfig{
		DAG: cfg,
	}}

	go func() {
		a.Run()
	}()

	time.Sleep(time.Millisecond * 30)

	status := a.Status()
	require.NotNil(t, status)
	require.Equal(t, status.Status, scheduler.SchedulerStatus_Running)

	_, err := testDAG(t, cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "is already running")
}

func TestDryRun(t *testing.T) {
	cfg := testLoadDAG(t, "agent_dry.yaml")

	a := &Agent{AgentConfig: &AgentConfig{
		DAG: cfg,
		Dry: true,
	}}
	err := a.Run()
	require.NoError(t, err)

	status := a.Status()
	require.NoError(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
}

func TestCancelDAG(t *testing.T) {
	for _, abort := range []func(*Agent){
		func(a *Agent) { a.Signal(syscall.SIGTERM) },
		func(a *Agent) { a.Cancel() },
	} {
		a, cfg := testDAGAsync(t, "agent_sleep.yaml")
		time.Sleep(time.Millisecond * 100)
		abort(a)
		time.Sleep(time.Millisecond * 500)
		status, err := controller.New(cfg).GetLastStatus()
		require.NoError(t, err)
		require.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	}
}

func TestPreConditionInvalid(t *testing.T) {
	cfg := testLoadDAG(t, "agent_multiple_steps.yaml")
	cfg.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "0",
		},
	}

	status, err := testDAG(t, cfg)
	require.Error(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Cancel, status.Status)
	require.Equal(t, scheduler.NodeStatus_None, status.Nodes[0].Status)
	require.Equal(t, scheduler.NodeStatus_None, status.Nodes[1].Status)
}

func TestPreConditionValid(t *testing.T) {
	cfg := testLoadDAG(t, "agent_with_params.yaml")

	cfg.Preconditions = []*config.Condition{
		{
			Condition: "`echo 1`",
			Expected:  "1",
		},
	}
	status, err := testDAG(t, cfg)
	require.NoError(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		require.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
}

func TestStartError(t *testing.T) {
	cfg := testLoadDAG(t, "agent_error.yaml")
	status, err := testDAG(t, cfg)
	require.Error(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Error, status.Status)
}

func TestOnExit(t *testing.T) {
	cfg := testLoadDAG(t, "agent_on_exit.yaml")
	status, err := testDAG(t, cfg)
	require.NoError(t, err)

	require.Equal(t, scheduler.SchedulerStatus_Success, status.Status)
	for _, s := range status.Nodes {
		require.Equal(t, scheduler.NodeStatus_Success, s.Status)
	}
	require.Equal(t, scheduler.NodeStatus_Success, status.OnExit.Status)
}

func TestRetry(t *testing.T) {
	cfg := testLoadDAG(t, "agent_retry.yaml")

	status, err := testDAG(t, cfg)
	require.Error(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Error, status.Status)

	for _, n := range status.Nodes {
		n.CmdWithArgs = "true"
	}
	a := &Agent{
		AgentConfig: &AgentConfig{
			DAG: cfg,
		},
		RetryConfig: &RetryConfig{
			Status: status,
		},
	}
	err = a.Run()
	status = a.Status()
	require.NoError(t, err)
	require.Equal(t, scheduler.SchedulerStatus_Success, status.Status)

	for _, n := range status.Nodes {
		if n.Status != scheduler.NodeStatus_Success &&
			n.Status != scheduler.NodeStatus_Skipped {
			t.Errorf("invalid status: %s", n.Status.String())
		}
	}
}

func TestHandleHTTP(t *testing.T) {
	cfg := testLoadDAG(t, "agent_handle_http.yaml")

	a := &Agent{AgentConfig: &AgentConfig{
		DAG: cfg,
	}}

	go func() {
		err := a.Run()
		require.NoError(t, err)
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

func testDAG(t *testing.T, cfg *config.Config) (*models.Status, error) {
	t.Helper()
	a := &Agent{AgentConfig: &AgentConfig{
		DAG: cfg,
	}}
	err := a.Run()
	return a.Status(), err
}

func testLoadDAG(t *testing.T, name string) *config.Config {
	file := path.Join(testsDir, name)
	cl := &config.Loader{}
	cfg, err := cl.Load(file, "")
	require.NoError(t, err)
	return cfg
}

func testDAGAsync(t *testing.T, file string) (*Agent, *config.Config) {
	t.Helper()

	cfg := testLoadDAG(t, file)
	a := &Agent{AgentConfig: &AgentConfig{
		DAG: cfg,
	}}

	go func() {
		a.Run()
	}()

	return a, cfg
}
