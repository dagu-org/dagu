// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/transform"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

const (
	apiTestProcHeartbeatInterval = 150 * time.Millisecond
	apiTestProcStaleThreshold    = time.Second
)

func TestServerProcHeartbeat_StartAPI(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Proc.HeartbeatInterval = apiTestProcHeartbeatInterval
		cfg.Proc.HeartbeatSyncInterval = apiTestProcHeartbeatInterval
		cfg.Proc.StaleThreshold = apiTestProcStaleThreshold
	}))

	spec := `
steps:
  - name: sleep
    command: sleep 6
`
	dagName := "api-proc-heartbeat"
	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	resp := server.Client().Post(fmt.Sprintf("/api/v1/dags/%s/start", dagName), api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).
		Send(t)

	var execResp api.ExecuteDAG200JSONResponse
	resp.Unmarshal(t, &execResp)
	require.NotEmpty(t, execResp.DagRunId)

	ref := exec.NewDAGRunRef(dagName, execResp.DagRunId)
	require.Eventually(t, func() bool {
		statusResp := server.Client().Get(fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)).
			ExpectStatus(http.StatusOK).
			Send(t)
		var details api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &details)
		return details.DagRun.Status == api.Status(core.Running)
	}, 5*time.Second, 50*time.Millisecond)

	time.Sleep(2 * time.Second)
	alive, err := server.ProcStore.IsRunAlive(server.Context, dagName, ref)
	require.NoError(t, err)
	require.True(t, alive)

	require.Eventually(t, func() bool {
		statusResp := server.Client().Get(fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)).
			ExpectStatus(http.StatusOK).
			Send(t)
		var details api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &details)
		return details.DagRun.Status == api.Status(core.Succeeded)
	}, 15*time.Second, 50*time.Millisecond)
}

func TestServerRepairsStaleLocalRunOnRead(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Proc.HeartbeatInterval = 50 * time.Millisecond
		cfg.Proc.HeartbeatSyncInterval = 50 * time.Millisecond
		cfg.Proc.StaleThreshold = 100 * time.Millisecond
	}))

	dag := server.DAG(t, `
name: api-stale-local-repair
steps:
  - name: step1
    command: sleep 2
`)

	dagRunID := uuid.Must(uuid.NewV7()).String()
	ref := exec.NewDAGRunRef(dag.Name, dagRunID)
	attempt, err := server.DAGRunStore.CreateAttempt(server.Context, dag.DAG, time.Now(), dagRunID, exec.NewDAGRunAttemptOptions{})
	require.NoError(t, err)

	logFile := filepath.Join(server.Config.Paths.LogDir, dag.Name, dagRunID+".log")
	require.NoError(t, os.MkdirAll(filepath.Dir(logFile), 0o750))

	status := transform.NewStatusBuilder(dag.DAG).Create(
		dagRunID,
		core.Running,
		0,
		time.Now().Add(-2*time.Second),
		transform.WithAttemptID(attempt.ID()),
		transform.WithHierarchyRefs(ref, exec.DAGRunRef{}),
		transform.WithLogFilePath(logFile),
	)
	require.NotEmpty(t, status.Nodes)
	status.Nodes[0].Status = core.NodeRunning

	require.NoError(t, attempt.Open(server.Context))
	require.NoError(t, attempt.Write(server.Context, status))
	require.NoError(t, attempt.Close(server.Context))

	_ = test.CreateStaleProcFile(
		t,
		server.Config.Paths.ProcDir,
		dag.ProcGroup(),
		ref,
		time.Now().Add(-2*time.Second),
		time.Second,
	)

	resp := server.Client().Get(fmt.Sprintf("/api/v1/dag-runs/%s/%s", dag.Name, dagRunID)).
		ExpectStatus(http.StatusOK).
		Send(t)

	var details api.GetDAGRunDetails200JSONResponse
	resp.Unmarshal(t, &details)
	require.Equal(t, api.Status(core.Failed), details.DagRunDetails.Status)

	repaired := test.ReadRunStatus(server.Context, t, server.DAGRunStore, ref)
	require.Equal(t, core.Failed, repaired.Status)
	require.Len(t, repaired.Nodes, 1)
	require.Equal(t, core.NodeFailed, repaired.Nodes[0].Status)
	require.Contains(t, repaired.Nodes[0].Error, "stale local process detected")
}
