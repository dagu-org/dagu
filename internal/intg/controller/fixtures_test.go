// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/controller"
	"github.com/dagucloud/dagu/internal/cmn/config"
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/core/exec"
	_ "github.com/dagucloud/dagu/internal/llm/allproviders"
	"github.com/dagucloud/dagu/internal/persis/fileagentconfig"
	"github.com/dagucloud/dagu/internal/persis/fileagentmodel"
	"github.com/dagucloud/dagu/internal/persis/filedagrun"
	"github.com/dagucloud/dagu/internal/persis/filememory"
	"github.com/dagucloud/dagu/internal/persis/filesession"
	"github.com/dagucloud/dagu/internal/persis/filewatermark"
	"github.com/dagucloud/dagu/internal/runtime"
	"github.com/dagucloud/dagu/internal/service/scheduler"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

const testModelID = "controller-intg-model"

type testFixture struct {
	t             *testing.T
	th            test.Command
	controller     *controller.Service
	llmServer     *fakeLLMServer
	scheduler     *scheduler.Scheduler
	schedulerCtx  context.Context
	cancelSched   context.CancelFunc
	schedulerErr  chan error
	schedulerSet  bool
	schedulerLast error
}

func controllerTestTimeout(timeout time.Duration) time.Duration {
	switch {
	case goruntime.GOOS == "windows" && raceEnabled():
		return timeout * 8
	case goruntime.GOOS == "windows":
		return timeout * 4
	case raceEnabled():
		return timeout * 2
	default:
		return timeout
	}
}

func newTestFixture(t *testing.T, dagName, dagYAML string, responses ...llmResponse) *testFixture {
	t.Helper()
	if goruntime.GOOS != "windows" && !raceEnabled() {
		t.Parallel()
	}

	llmServer := newFakeLLMServer(t, responses...)

	th := test.SetupCommand(t,
		test.WithBuiltExecutable(),
		test.WithConfigMutator(func(c *config.Config) {
			c.Queues.Enabled = true
			c.Scheduler.Port = 0
		}),
	)
	th.CreateDAGFile(t, dagName+".yaml", dagYAML)

	configStore, err := fileagentconfig.New(th.Config.Paths.DataDir)
	require.NoError(t, err)
	agentCfg := agent.DefaultConfig()
	agentCfg.Enabled = true
	agentCfg.DefaultModelID = testModelID
	require.NoError(t, configStore.Save(th.Context, agentCfg))

	modelStore, err := fileagentmodel.New(filepath.Join(th.Config.Paths.DataDir, "agent", "models"))
	require.NoError(t, err)
	require.NoError(t, modelStore.Create(th.Context, &agent.ModelConfig{
		ID:       testModelID,
		Name:     "Controller Integration Model",
		Provider: "local",
		Model:    "controller-intg",
		BaseURL:  llmServer.baseURL(),
	}))

	sessionStore, err := filesession.New(th.Config.Paths.SessionsDir)
	require.NoError(t, err)
	memoryStore, err := filememory.New(th.Config.Paths.DAGsDir)
	require.NoError(t, err)

	agentAPI := agent.NewAPI(agent.APIConfig{
		ConfigStore:  configStore,
		ModelStore:   modelStore,
		WorkingDir:   th.Config.Paths.DAGsDir,
		SessionStore: sessionStore,
		DAGStore:     th.DAGStore,
		MemoryStore:  memoryStore,
		Environment: agent.EnvironmentInfo{
			DAGsDir:        th.Config.Paths.DAGsDir,
			DocsDir:        th.Config.Paths.DocsDir,
			LogDir:         th.Config.Paths.LogDir,
			DataDir:        th.Config.Paths.DataDir,
			ConfigFile:     th.Config.Paths.ConfigFileUsed,
			WorkingDir:     th.Config.Paths.DAGsDir,
			BaseConfigFile: th.Config.Paths.BaseConfig,
		},
	})

	controllerService := controller.New(
		th.Config,
		th.DAGStore,
		th.DAGRunStore,
		controller.WithSessionStore(sessionStore),
		controller.WithMemoryStore(memoryStore),
		controller.WithDAGRunController(&th.DAGRunMgr),
		controller.WithAgentAPI(agentAPI),
		controller.WithSubCmdBuilder(runtime.NewSubCmdBuilder(th.Config)),
	)

	f := &testFixture{
		t:         t,
		th:        th,
		controller: controllerService,
		llmServer: llmServer,
	}
	t.Cleanup(f.cleanup)
	return f
}

func (f *testFixture) putController(name, dagName string) {
	f.t.Helper()
	require.NoError(f.t, f.controller.PutSpec(f.th.Context, name, `description: Integration test Controller
goal: Run the workflow
workflows:
  names:
    - `+dagName+`
agent:
  model: `+testModelID+`
`))
}

func (f *testFixture) startController(name string) {
	f.t.Helper()
	_, err := f.controller.CreateTask(f.th.Context, name, controller.CreateTaskRequest{
		Description: "Run the integration test DAG",
		RequestedBy: "integration-test",
	})
	require.NoError(f.t, err)
	require.NoError(f.t, f.controller.RequestStart(f.th.Context, name, controller.StartRequest{
		RequestedBy: "integration-test",
		Instruction: "Run the workflow now.",
	}))
	require.NoError(f.t, f.controller.ReconcileOnce(f.th.Context))
}

func (f *testFixture) waitForCurrentRun(name string, timeout time.Duration) exec.DAGRunRef {
	f.t.Helper()
	timeout = controllerTestTimeout(timeout)

	var ref exec.DAGRunRef
	var lastErr error
	require.Eventually(f.t, func() bool {
		if err := f.pollSchedulerErr(); err != nil {
			lastErr = err
			return true
		}
		detail, err := f.controller.Detail(f.th.Context, name)
		if err != nil {
			lastErr = err
			return false
		}
		if detail.State == nil || detail.State.CurrentRunRef == nil {
			return false
		}
		ref = *detail.State.CurrentRunRef
		return true
	}, timeout, 100*time.Millisecond)
	require.NoError(f.t, lastErr)
	return ref
}

func (f *testFixture) startScheduler(timeout time.Duration) {
	f.t.Helper()
	if f.scheduler != nil {
		return
	}

	entryReader := scheduler.NewEntryReader(f.th.Config.Paths.DAGsDir, f.th.DAGStore)
	schedulerInst, err := scheduler.New(
		f.th.Config,
		entryReader,
		f.th.DAGRunMgr,
		f.th.DAGRunStore,
		f.th.QueueStore,
		f.th.ProcStore,
		f.th.ServiceRegistry,
		nil,
		filewatermark.New(filepath.Join(f.th.Config.Paths.DataDir, "scheduler")),
	)
	require.NoError(f.t, err)
	schedulerInst.SetDAGRunLeaseStore(f.th.DAGRunLeaseStore)

	ctx, cancel := context.WithCancel(f.th.Context)
	f.scheduler = schedulerInst
	f.schedulerCtx = ctx
	f.cancelSched = cancel
	f.schedulerErr = make(chan error, 1)

	go func() {
		f.schedulerErr <- schedulerInst.Start(ctx)
	}()

	startupTimeout := controllerTestTimeout(timeout)
	require.Eventually(f.t, func() bool {
		if err := f.pollSchedulerErr(); err != nil {
			return true
		}
		return schedulerInst.IsRunning()
	}, startupTimeout, 50*time.Millisecond, "scheduler should start")
	require.NoError(f.t, f.schedulerLast)
}

func (f *testFixture) waitForStatus(ref exec.DAGRunRef, expected core.Status, timeout time.Duration) *exec.DAGRunStatus {
	f.t.Helper()
	timeout = controllerTestTimeout(timeout)

	var status *exec.DAGRunStatus
	var lastErr error
	require.Eventually(f.t, func() bool {
		if err := f.pollSchedulerErr(); err != nil {
			lastErr = err
			return true
		}
		status, lastErr = f.status(ref)
		return lastErr == nil && status != nil && status.Status == expected
	}, timeout, 100*time.Millisecond, "timeout waiting for %s to reach %s", ref, expected)
	require.NoError(f.t, lastErr)
	return status
}

func (f *testFixture) status(ref exec.DAGRunRef) (*exec.DAGRunStatus, error) {
	store := filedagrun.New(
		f.th.Config.Paths.DAGRunsDir,
		filedagrun.WithLatestStatusToday(f.th.Config.Server.LatestStatusToday),
		filedagrun.WithLocation(f.th.Config.Core.Location),
	)
	attempt, err := store.FindAttempt(f.th.Context, ref)
	if err != nil {
		return nil, err
	}
	return attempt.ReadStatus(f.th.Context)
}

func (f *testFixture) storedDAG(ref exec.DAGRunRef) *core.DAG {
	f.t.Helper()
	attempt, err := f.th.DAGRunStore.FindAttempt(f.th.Context, ref)
	require.NoError(f.t, err)
	dag, err := attempt.ReadDAG(f.th.Context)
	require.NoError(f.t, err)
	return dag
}

func (f *testFixture) controllerWorkspace(name string) string {
	return filepath.Join(f.th.Config.Paths.DataDir, "controller", name, "workspace")
}

func (f *testFixture) pollSchedulerErr() error {
	if f.schedulerSet {
		return f.schedulerLast
	}
	if f.schedulerErr == nil {
		return nil
	}
	select {
	case err := <-f.schedulerErr:
		f.schedulerSet = true
		if err == nil {
			err = errors.New("scheduler exited unexpectedly")
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			err = nil
		}
		f.schedulerLast = err
	default:
		return nil
	}
	return f.schedulerLast
}

func (f *testFixture) cleanup() {
	f.t.Helper()
	if f.cancelSched != nil {
		f.cancelSched()
	}
	if f.scheduler != nil {
		f.scheduler.Stop(context.Background())
	}
	if f.schedulerErr != nil && !f.schedulerSet {
		select {
		case err := <-f.schedulerErr:
			if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
				f.t.Logf("scheduler stopped with error: %v", err)
			}
		case <-time.After(5 * time.Second):
			f.t.Log("scheduler did not stop within 5 seconds")
		}
	}
}

type llmResponse struct {
	content      string
	finishReason string
	toolCalls    []llmToolCall
}

type llmToolCall struct {
	id        string
	name      string
	arguments string
}

func toolCallResponse(name, arguments string) llmResponse {
	return llmResponse{
		finishReason: "tool_calls",
		toolCalls: []llmToolCall{{
			id:        "call-1",
			name:      name,
			arguments: arguments,
		}},
	}
}

func stopResponse(content string) llmResponse {
	return llmResponse{
		content:      content,
		finishReason: "stop",
	}
}

type fakeLLMServer struct {
	t         *testing.T
	server    *httptest.Server
	mu        sync.Mutex
	responses []llmResponse
	requests  []chatCompletionRequest
}

func newFakeLLMServer(t *testing.T, responses ...llmResponse) *fakeLLMServer {
	t.Helper()
	s := &fakeLLMServer{
		t:         t,
		responses: append([]llmResponse(nil), responses...),
	}
	s.server = httptest.NewServer(http.HandlerFunc(s.handleChat))
	t.Cleanup(s.server.Close)
	return s
}

func (s *fakeLLMServer) baseURL() string {
	return s.server.URL + "/v1"
}

func (s *fakeLLMServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/v1/chat/completions" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var req chatCompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp := stopResponse("done")
	s.mu.Lock()
	s.requests = append(s.requests, req)
	if len(s.responses) > 0 {
		resp = s.responses[0]
		s.responses = s.responses[1:]
	}
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(chatCompletionResponse(resp))
}

type chatCompletionRequest struct {
	Model    string `json:"model"`
	Messages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	} `json:"messages"`
	Tools []struct {
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	} `json:"tools"`
}

func chatCompletionResponse(resp llmResponse) map[string]any {
	message := map[string]any{
		"role":    "assistant",
		"content": resp.content,
	}
	if len(resp.toolCalls) > 0 {
		calls := make([]map[string]any, 0, len(resp.toolCalls))
		for _, call := range resp.toolCalls {
			id := call.id
			if id == "" {
				id = "call-1"
			}
			calls = append(calls, map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      call.name,
					"arguments": call.arguments,
				},
			})
		}
		message["tool_calls"] = calls
	}

	return map[string]any{
		"id":      "chatcmpl-test",
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "controller-intg",
		"choices": []map[string]any{{
			"index":         0,
			"message":       message,
			"finish_reason": resp.finishReason,
		}},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	}
}

func controllerRunWorkflowArgs(dagName string) string {
	return fmt.Sprintf(`{"workflow_name":%q}`, dagName)
}

func workingDirProbeDAG(name string) string {
	return `name: ` + name + `
steps:
  - name: write-pwd
    type: command
` + workingDirProbeCommand()
}

func explicitWorkingDirProbeDAG(name, workingDir string) string {
	return `name: ` + name + `
working_dir: ` + yamlSingleQuote(workingDir) + `
steps:
  - name: write-pwd
    type: command
` + workingDirProbeCommand()
}

func workingDirProbeCommand() string {
	if goruntime.GOOS == "windows" {
		return `    shell: powershell
    command: |
      (Get-Location).Path | Set-Content -NoNewline actual_pwd.txt
`
	}
	return `    shell: /bin/sh
    command: |
      pwd -P > actual_pwd.txt
`
}

func yamlSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func readTrimmedFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return strings.TrimSpace(string(data))
}

func assertSamePath(t *testing.T, expected, actual string) {
	t.Helper()
	expected = cleanPhysicalPath(t, expected)
	actual = cleanPhysicalPath(t, actual)
	if goruntime.GOOS == "windows" {
		require.Truef(t, strings.EqualFold(expected, actual), "expected path %q to equal %q", actual, expected)
		return
	}
	require.Equal(t, expected, actual)
}

func cleanPhysicalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	}
	return filepath.Clean(path)
}
