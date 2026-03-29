// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

type dagListResponse struct {
	Dags []struct {
		FileName string `json:"fileName"`
		DAG      struct {
			Name string   `json:"name"`
			Tags []string `json:"tags"`
		} `json:"dag"`
	} `json:"dags"`
}

type dagRunsResponse struct {
	DagRuns []struct {
		Name     string   `json:"name"`
		DagRunID string   `json:"dagRunId"`
		Tags     []string `json:"tags"`
	} `json:"dagRuns"`
}

type workspacesResponse struct {
	Workspaces []struct {
		Name string `json:"name"`
	} `json:"workspaces"`
}

func TestWorkspaceAwareAPIEndpoints(t *testing.T) {
	t.Run("dagsListRespectsWorkspaceTagFiltering", func(t *testing.T) {
		server := test.SetupServer(t)

		server.CreateDAGFile(t, server.Config.Paths.DAGsDir, "platform.yaml", []byte(`name: deploy-platform
tags:
  - workspace=platform
steps:
  - name: deploy
    command: echo platform
`))
		server.CreateDAGFile(t, server.Config.Paths.DAGsDir, "ops.yaml", []byte(`name: deploy-ops
tags:
  - workspace=ops
steps:
  - name: deploy
    command: echo ops
`))
		server.CreateDAGFile(t, server.Config.Paths.DAGsDir, "shared.yaml", []byte(`name: shared-job
steps:
  - name: run
    command: echo shared
`))

		resp := server.Client().Get("/api/v1/dags?tags=workspace=platform").
			ExpectStatus(200).
			Send(t)

		var body dagListResponse
		resp.Unmarshal(t, &body)
		require.Len(t, body.Dags, 1)
		require.Equal(t, "deploy-platform", body.Dags[0].DAG.Name)
		require.Equal(t, []string{"workspace=platform"}, body.Dags[0].DAG.Tags)
	})

	t.Run("dagRunsListAndExecutionTagsStayWorkspaceScoped", func(t *testing.T) {
		server := test.SetupServer(t)

		server.CreateDAGFile(t, server.Config.Paths.DAGsDir, "runs.yaml", []byte(`name: workspace-runs
steps:
  - name: run
    command: echo hello
`))

		startResp := server.Client().
			Post("/api/v1/dags/runs.yaml/start", map[string]any{
				"tags": []string{"workspace=platform"},
			}).
			ExpectStatus(200).
			Send(t)
		var startBody struct {
			DagRunID string `json:"dagRunId"`
		}
		startResp.Unmarshal(t, &startBody)
		require.NotEmpty(t, startBody.DagRunID)

		enqueueResp := server.Client().
			Post("/api/v1/dags/runs.yaml/enqueue", map[string]any{
				"tags": []string{"workspace=ops"},
			}).
			ExpectStatus(200).
			Send(t)
		var enqueueBody struct {
			DagRunID string `json:"dagRunId"`
		}
		enqueueResp.Unmarshal(t, &enqueueBody)
		require.NotEmpty(t, enqueueBody.DagRunID)

		require.Eventually(t, func() bool {
			resp := server.Client().
				Get("/api/v1/dag-runs?tags=workspace=platform").
				ExpectStatus(200).
				Send(t)
			var body dagRunsResponse
			resp.Unmarshal(t, &body)
			return len(body.DagRuns) == 1 && body.DagRuns[0].DagRunID == startBody.DagRunID
		}, 10*time.Second, 200*time.Millisecond)

		resp := server.Client().
			Get("/api/v1/dag-runs?tags=workspace=ops").
			ExpectStatus(200).
			Send(t)
		var opsRuns dagRunsResponse
		resp.Unmarshal(t, &opsRuns)
		require.Len(t, opsRuns.DagRuns, 1)
		require.Equal(t, enqueueBody.DagRunID, opsRuns.DagRuns[0].DagRunID)
		require.Equal(t, []string{"workspace=ops"}, opsRuns.DagRuns[0].Tags)
	})

	t.Run("workspacesRemainRemoteNodeSpecific", func(t *testing.T) {
		remoteServer := test.SetupServer(t)
		remoteAPIURL := fmt.Sprintf(
			"http://%s:%d/api/v1",
			remoteServer.Config.Server.Host,
			remoteServer.Config.Server.Port,
		)

		localServer := test.SetupServer(
			t,
			test.WithConfigMutator(func(cfg *config.Config) {
				cfg.Server.RemoteNodes = []config.RemoteNode{
					{
						Name:       "dev",
						APIBaseURL: remoteAPIURL,
					},
				}
			}),
		)

		localServer.Client().
			Post("/api/v1/workspaces", map[string]any{"name": "local-ops"}).
			ExpectStatus(201).
			Send(t)
		remoteServer.Client().
			Post("/api/v1/workspaces", map[string]any{"name": "remote-qa"}).
			ExpectStatus(201).
			Send(t)

		localResp := localServer.Client().
			Get("/api/v1/workspaces").
			ExpectStatus(200).
			Send(t)
		var localBody workspacesResponse
		localResp.Unmarshal(t, &localBody)
		require.Len(t, localBody.Workspaces, 1)
		require.Equal(t, "local-ops", localBody.Workspaces[0].Name)

		remoteResp := localServer.Client().
			Get("/api/v1/workspaces?remoteNode=dev").
			ExpectStatus(200).
			Send(t)
		var remoteBody workspacesResponse
		remoteResp.Unmarshal(t, &remoteBody)
		require.Len(t, remoteBody.Workspaces, 1)
		require.Equal(t, "remote-qa", remoteBody.Workspaces[0].Name)
	})
}
