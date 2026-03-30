// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestBatchRetryRetriesRealPersistedRuns(t *testing.T) {
	server := test.SetupServer(t)

	retryCommand := `
if [ -f "$DAG_RUN_LOG_FILE.marker" ]; then
  echo retry-success
else
  touch "$DAG_RUN_LOG_FILE.marker"
  exit 1
fi
`
	if runtime.GOOS == "windows" {
		retryCommand = `
if (Test-Path "$env:DAG_RUN_LOG_FILE.marker") {
  Write-Output retry-success
} else {
  New-Item -ItemType File -Path "$env:DAG_RUN_LOG_FILE.marker" -Force | Out-Null
  exit 1
}
`
	}

	spec := fmt.Sprintf(`
steps:
  - name: main
    command: |
%s
`, indentBlock(retryCommand, 6))

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "intg_batch_retry_dag",
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	runIDs := []string{
		startRun(t, server, "intg_batch_retry_dag", nil),
		startRun(t, server, "intg_batch_retry_dag", nil),
	}

	for _, runID := range runIDs {
		waitForRunStatus(t, server, "intg_batch_retry_dag", runID, core.Failed, 15*time.Second)
	}

	resp := server.Client().Post("/api/v1/dag-runs/retry-batch", api.RetryDAGRunsBatchJSONRequestBody{
		Items: []api.DAGRunBatchActionItem{
			{Name: "intg_batch_retry_dag", DagRunId: runIDs[0]},
			{Name: "intg_batch_retry_dag", DagRunId: runIDs[1]},
		},
	}).ExpectStatus(http.StatusOK).Send(t)

	var body api.RetryDAGRunsBatch200JSONResponse
	resp.Unmarshal(t, &body)
	require.Equal(t, 2, body.SuccessCount)
	require.Equal(t, 0, body.FailureCount)

	for _, runID := range runIDs {
		waitForRunStatus(t, server, "intg_batch_retry_dag", runID, core.Succeeded, 20*time.Second)
	}
}

func TestBatchRescheduleReschedulesMultipleHistoricalRunsFromSameDAG(t *testing.T) {
	server := test.SetupServer(t)

	spec := `
params: "value"
steps:
  - name: main
    command: echo "${value}"
`

	_ = server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: "intg_batch_reschedule_dag",
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	paramsA := `{"value":"alpha"}`
	paramsB := `{"value":"beta"}`

	sourceRuns := []struct {
		runID  string
		params string
	}{
		{runID: startRun(t, server, "intg_batch_reschedule_dag", &paramsA), params: paramsA},
		{runID: startRun(t, server, "intg_batch_reschedule_dag", &paramsB), params: paramsB},
	}

	originalParams := make(map[string]string, len(sourceRuns))
	for _, source := range sourceRuns {
		details := waitForRunStatus(t, server, "intg_batch_reschedule_dag", source.runID, core.Succeeded, 15*time.Second)
		require.NotNil(t, details.DagRun.Params)
		originalParams[source.runID] = *details.DagRun.Params
	}

	resp := server.Client().Post("/api/v1/dag-runs/reschedule-batch", api.RescheduleDAGRunsBatchJSONRequestBody{
		Items: []api.DAGRunBatchActionItem{
			{Name: "intg_batch_reschedule_dag", DagRunId: sourceRuns[0].runID},
			{Name: "intg_batch_reschedule_dag", DagRunId: sourceRuns[1].runID},
		},
	}).ExpectStatus(http.StatusOK).Send(t)

	var body api.RescheduleDAGRunsBatch200JSONResponse
	resp.Unmarshal(t, &body)
	require.Equal(t, 2, body.SuccessCount)
	require.Equal(t, 0, body.FailureCount)
	require.Len(t, body.Results, 2)

	for i, result := range body.Results {
		require.True(t, result.Ok, "result %d should succeed", i)
		require.NotNil(t, result.NewDagRunId)

		rescheduled := waitForRunStatus(t, server, "intg_batch_reschedule_dag", *result.NewDagRunId, core.Succeeded, 15*time.Second)
		require.NotNil(t, rescheduled.DagRun.Params)
		require.Equal(t, originalParams[sourceRuns[i].runID], *rescheduled.DagRun.Params)
	}
}

func startRun(t *testing.T, server test.Server, dagName string, params *string) string {
	t.Helper()

	resp := server.Client().Post("/api/v1/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
		Params: params,
	}).ExpectStatus(http.StatusOK).Send(t)

	var body api.ExecuteDAG200JSONResponse
	resp.Unmarshal(t, &body)
	require.NotEmpty(t, body.DagRunId)
	return body.DagRunId
}

func waitForRunStatus(
	t *testing.T,
	server test.Server,
	dagName, dagRunID string,
	expected core.Status,
	timeout time.Duration,
) api.GetDAGDAGRunDetails200JSONResponse {
	t.Helper()

	var details api.GetDAGDAGRunDetails200JSONResponse
	require.Eventually(t, func() bool {
		resp := server.Client().Get(fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, dagRunID)).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		resp.Unmarshal(t, &details)
		return details.DagRun.Status == api.Status(expected)
	}, timeout, 200*time.Millisecond)

	return details
}

func indentBlock(s string, spaces int) string {
	prefix := fmt.Sprintf("%*s", spaces, "")
	var result strings.Builder
	for _, line := range splitLines(s) {
		if line == "" {
			result.WriteString(prefix + "\n")
			continue
		}
		result.WriteString(prefix + line + "\n")
	}
	return result.String()
}

func splitLines(s string) []string {
	lines := []string{}
	current := ""
	for _, r := range s {
		if r == '\n' {
			lines = append(lines, current)
			current = ""
			continue
		}
		current += string(r)
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
