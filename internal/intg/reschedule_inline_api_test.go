// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	exec1 "github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/service/coordinator"
	"github.com/dagu-org/dagu/internal/service/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestAPIRescheduleInlineStartUsesStoredSnapshot(t *testing.T) {
	server := test.SetupServer(t)

	runID, location := createInlineServerRunForReschedule(t, server, false)
	requireMissingFile(t, location)

	newRunID := rescheduleServerInlineRun(t, server, "intg_inline_reschedule_start", runID)
	requireServerRunParams(t, server, "intg_inline_reschedule_start", newRunID)
}

func TestAPIRescheduleInlineEnqueueUsesStoredSnapshot(t *testing.T) {
	server := test.SetupServer(t)

	runID, location := createInlineServerRunForReschedule(t, server, true)
	requireMissingFile(t, location)

	newRunID := rescheduleServerInlineRun(t, server, "intg_inline_reschedule_enqueue", runID)
	requireServerRunParams(t, server, "intg_inline_reschedule_enqueue", newRunID)
}

func createInlineServerRunForReschedule(t *testing.T, server test.Server, enqueue bool) (string, string) {
	t.Helper()

	specContent := `params:
  - name: KEY
    default: fallback
  - name: COUNT
    default: 1
steps:
  - name: print
    command: echo "${KEY}|${COUNT}"`
	name := "intg_inline_reschedule_start"
	if enqueue {
		name = "intg_inline_reschedule_enqueue"
	}
	params := `KEY="hello world" COUNT=3`

	var runID string
	if enqueue {
		resp := server.Client().Post("/api/v1/dag-runs/enqueue", api.EnqueueDAGRunFromSpecJSONRequestBody{
			Spec:   specContent,
			Name:   &name,
			Params: &params,
		}).ExpectStatus(http.StatusOK).Send(t)

		var body api.EnqueueDAGRunFromSpec200JSONResponse
		resp.Unmarshal(t, &body)
		runID = body.DagRunId
	} else {
		resp := server.Client().Post("/api/v1/dag-runs", api.ExecuteDAGRunFromSpecJSONRequestBody{
			Spec:   specContent,
			Name:   &name,
			Params: &params,
		}).ExpectStatus(http.StatusOK).Send(t)

		var body api.ExecuteDAGRunFromSpec200JSONResponse
		resp.Unmarshal(t, &body)
		runID = body.DagRunId
	}
	require.NotEmpty(t, runID)

	if enqueue {
		processQueuedServerInlineRun(t, server, name)
		require.Eventually(t, func() bool {
			statusAttempt := waitForServerAttempt(t, server, name, runID)
			status, err := statusAttempt.ReadStatus(server.Context)
			return err == nil && status.Status == core.Succeeded
		}, 10*time.Second, 200*time.Millisecond)
	}

	attempt, dag := waitForServerAttemptWithDAG(t, server, name, runID)
	require.Contains(t, string(dag.YamlData), `echo "${KEY}|${COUNT}"`)

	status, err := attempt.ReadStatus(server.Context)
	require.NoError(t, err)
	require.Equal(t, []string{"KEY=hello world", "COUNT=3"}, status.ParamsList)

	location := dag.Location
	if location == "" {
		location = expectedServerInlineTempPath(name, runID)
	}

	return runID, location
}

func rescheduleServerInlineRun(t *testing.T, server test.Server, dagName, runID string) string {
	t.Helper()

	resp := server.Client().Post(
		fmt.Sprintf("/api/v1/dag-runs/%s/%s/reschedule", dagName, runID),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var body api.RescheduleDAGRun200JSONResponse
	resp.Unmarshal(t, &body)
	require.NotEmpty(t, body.DagRunId)
	return body.DagRunId
}

func requireServerRunParams(t *testing.T, server test.Server, dagName, runID string) {
	t.Helper()

	attempt := waitForServerAttempt(t, server, dagName, runID)
	require.Eventually(t, func() bool {
		status, err := attempt.ReadStatus(server.Context)
		require.NoError(t, err)
		return status.Status == core.Succeeded
	}, 10*time.Second, 200*time.Millisecond)

	status, err := attempt.ReadStatus(server.Context)
	require.NoError(t, err)
	require.Equal(t, []string{"KEY=hello world", "COUNT=3"}, status.ParamsList)
}

func waitForServerAttempt(t *testing.T, server test.Server, dagName, runID string) exec1.DAGRunAttempt {
	t.Helper()

	var attempt exec1.DAGRunAttempt
	require.Eventually(t, func() bool {
		var err error
		attempt, err = server.DAGRunStore.FindAttempt(server.Context, exec1.NewDAGRunRef(dagName, runID))
		return err == nil
	}, 10*time.Second, 100*time.Millisecond)

	return attempt
}

func waitForServerAttemptWithDAG(t *testing.T, server test.Server, dagName, runID string) (exec1.DAGRunAttempt, *core.DAG) {
	t.Helper()

	attempt := waitForServerAttempt(t, server, dagName, runID)

	var dag *core.DAG
	require.Eventually(t, func() bool {
		var err error
		dag, err = attempt.ReadDAG(server.Context)
		return err == nil && dag != nil && len(dag.YamlData) > 0
	}, 10*time.Second, 100*time.Millisecond)

	return attempt, dag
}

func expectedServerInlineTempPath(name, dagRunID string) string {
	return filepath.Join(os.TempDir(), name, dagRunID, fmt.Sprintf("%s.yaml", name))
}

func processQueuedServerInlineRun(t *testing.T, server test.Server, queueName string) {
	t.Helper()

	queueProcessor := scheduler.NewQueueProcessor(
		server.QueueStore,
		server.DAGRunStore,
		server.ProcStore,
		scheduler.NewDAGExecutor(
			coordinator.New(server.ServiceRegistry, coordinator.DefaultConfig()),
			server.SubCmdBuilder,
			server.Config.DefaultExecMode,
			server.Config.Paths.BaseConfig,
		),
		config.Queues{
			Enabled: true,
			Config: []config.QueueConfig{
				{Name: queueName, MaxActiveRuns: 1},
			},
		},
	)
	queueProcessor.ProcessQueueItems(server.Context, queueName)
}

func requireMissingFile(t *testing.T, path string) {
	t.Helper()

	_, err := os.Stat(path)
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
}
