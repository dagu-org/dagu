// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRecentEventsLocalLifecycleFeed(t *testing.T) {
	t.Run("WaitingAndApproved", func(t *testing.T) {
		server := test.SetupServer(t)
		dagRunID := createRecentEventsDAGRun(t, server, "recent_events_wait_approve", `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo done"`)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:            dagRunID,
			eventType:           "waiting",
			stepName:            "wait-step",
			resultingRunStatus:  "waiting",
			expectApprovalCycle: nil,
		})

		server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/recent_events_wait_approve/%s/steps/wait-step/approve", dagRunID),
			api.ApproveStepRequest{},
		).ExpectStatus(http.StatusOK).Send(t)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:           dagRunID,
			eventType:          "approved",
			stepName:           "wait-step",
			resultingRunStatus: "running",
			resumed:            new(true),
		})
	})

	t.Run("Rejected", func(t *testing.T) {
		server := test.SetupServer(t)
		dagRunID := createRecentEventsDAGRun(t, server, "recent_events_reject", `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"`)

		waitForDagRunStatus(t, server, "recent_events_reject", dagRunID, api.StatusWaiting)

		reason := "not today"
		server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/recent_events_reject/%s/steps/wait-step/reject", dagRunID),
			api.RejectStepRequest{Reason: &reason},
		).ExpectStatus(http.StatusOK).Send(t)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:           dagRunID,
			eventType:          "rejected",
			stepName:           "wait-step",
			reason:             reason,
			resultingRunStatus: "rejected",
		})
	})

	t.Run("PushBack", func(t *testing.T) {
		server := test.SetupServer(t)
		dagRunID := createRecentEventsDAGRun(t, server, "recent_events_push_back", `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo done"`)

		waitForDagRunStatus(t, server, "recent_events_push_back", dagRunID, api.StatusWaiting)

		server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/recent_events_push_back/%s/steps/wait-step/push-back", dagRunID),
			api.PushBackStepRequest{},
		).ExpectStatus(http.StatusOK).Send(t)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:            dagRunID,
			eventType:           "push_back",
			stepName:            "wait-step",
			resultingRunStatus:  "running",
			resumed:             new(true),
			expectApprovalCycle: new(1),
		})
	})

	t.Run("Failed", func(t *testing.T) {
		server := test.SetupServer(t)
		dagRunID := createRecentEventsDAGRun(t, server, "recent_events_failed", `type: graph
steps:
  - name: boom
    command: "sh -c 'echo fail >&2; exit 1'"`)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:           dagRunID,
			eventType:          "failed",
			stepName:           "boom",
			resultingRunStatus: "failed",
		})
	})

	t.Run("Aborted", func(t *testing.T) {
		server := test.SetupServer(t)
		dagRunID := createRecentEventsDAGRun(t, server, "recent_events_aborted", `type: graph
steps:
  - name: long-step
    command: "sleep 30"`)

		waitForDagRunStatus(t, server, "recent_events_aborted", dagRunID, api.StatusRunning)
		stopDagRunLocally(t, server, "recent_events_aborted", dagRunID)

		waitForEvent(t, server, recentEventExpectation{
			dagRunID:           dagRunID,
			eventType:          "aborted",
			stepName:           "long-step",
			resultingRunStatus: "aborted",
		})
	})
}

type recentEventExpectation struct {
	dagRunID            string
	eventType           string
	stepName            string
	reason              string
	resultingRunStatus  string
	resumed             *bool
	expectApprovalCycle *int
}

func createRecentEventsDAGRun(t *testing.T, server test.Server, dagName, spec string) string {
	t.Helper()

	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post(fmt.Sprintf("/api/v1/dags/%s/start", dagName), api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)
	return startBody.DagRunId
}

func waitForDagRunStatus(t *testing.T, server test.Server, dagName, dagRunID string, expected api.Status) {
	t.Helper()

	require.Eventually(t, func() bool {
		resp := server.Client().Get(fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, dagRunID)).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var body api.GetDAGDAGRunDetails200JSONResponse
		resp.Unmarshal(t, &body)
		return body.DagRun.Status == expected
	}, 20*time.Second, 100*time.Millisecond)
}

func waitForEvent(t *testing.T, server test.Server, want recentEventExpectation) {
	t.Helper()

	require.Eventually(t, func() bool {
		values := url.Values{}
		values.Set("dagRunId", want.dagRunID)
		values.Set("type", want.eventType)

		resp := server.Client().Get("/api/v1/recent-events?" + values.Encode()).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var body api.RecentEventsResponse
		resp.Unmarshal(t, &body)
		for _, entry := range body.Entries {
			if entry.DagRunId != want.dagRunID || string(entry.Type) != want.eventType {
				continue
			}
			if want.stepName != "" {
				if entry.StepName == nil || *entry.StepName != want.stepName {
					continue
				}
			}
			if want.reason != "" {
				if entry.Reason == nil || *entry.Reason != want.reason {
					continue
				}
			}
			if want.resultingRunStatus != "" {
				if entry.ResultingRunStatus == nil || *entry.ResultingRunStatus != want.resultingRunStatus {
					continue
				}
			}
			if want.resumed != nil {
				if entry.Resumed == nil || *entry.Resumed != *want.resumed {
					continue
				}
			}
			if want.expectApprovalCycle != nil {
				if entry.ApprovalIteration == nil || *entry.ApprovalIteration != *want.expectApprovalCycle {
					continue
				}
			}
			return true
		}
		return false
	}, 20*time.Second, 100*time.Millisecond)
}

func stopDagRunLocally(t *testing.T, server test.Server, dagName, dagRunID string) {
	t.Helper()

	attempt, err := server.DAGRunStore.FindAttempt(server.Context, exec.NewDAGRunRef(dagName, dagRunID))
	require.NoError(t, err)

	dag, err := attempt.ReadDAG(server.Context)
	require.NoError(t, err)

	require.NoError(t, server.DAGRunMgr.Stop(server.Context, dag, dagRunID))
}
