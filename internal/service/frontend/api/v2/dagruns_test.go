package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRescheduleDAGRun(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo reschedule"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_dag",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v2/dags/reschedule_dag/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_dag/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	rescheduleResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/reschedule", "reschedule_dag", startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{},
	).ExpectStatus(http.StatusOK).Send(t)

	var rescheduleBody api.RescheduleDAGRun200JSONResponse
	rescheduleResp.Unmarshal(t, &rescheduleBody)
	require.NotEmpty(t, rescheduleBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_dag/dag-runs/%s", rescheduleBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)
}

func TestRescheduleDAGRun_InvalidDefinitionStrategy(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: main
    command: "echo reschedule"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_invalid",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post("/api/v2/dags/reschedule_invalid/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_invalid/dag-runs/%s", startBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 10*time.Second, 200*time.Millisecond)

	strategy := api.RescheduleDAGRunJSONBodyDefinitionStrategyLatest
	errorResp := server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/reschedule", "reschedule_invalid", startBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{DefinitionStrategy: &strategy},
	).ExpectStatus(http.StatusBadRequest).Send(t)

	require.Contains(t, errorResp.Body, "not supported")
}

func TestRescheduleDAGRun_SingletonConflict(t *testing.T) {
	server := test.SetupServer(t)

	dagSpec := `steps:
  - name: slow
    command: "bash -lc 'sleep 5'"`

	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "reschedule_singleton_conflict",
		Spec: &dagSpec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	firstRunResp := server.Client().Post("/api/v2/dags/reschedule_singleton_conflict/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var firstRunBody api.ExecuteDAG200JSONResponse
	firstRunResp.Unmarshal(t, &firstRunBody)
	require.NotEmpty(t, firstRunBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_singleton_conflict/dag-runs/%s", firstRunBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 15*time.Second, 200*time.Millisecond)

	secondRunResp := server.Client().Post("/api/v2/dags/reschedule_singleton_conflict/start", api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var secondRunBody api.ExecuteDAG200JSONResponse
	secondRunResp.Unmarshal(t, &secondRunBody)
	require.NotEmpty(t, secondRunBody.DagRunId)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_singleton_conflict/dag-runs/%s", secondRunBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Running)
	}, 5*time.Second, 200*time.Millisecond)

	singleton := true
	server.Client().Post(
		fmt.Sprintf("/api/v2/dag-runs/%s/%s/reschedule", "reschedule_singleton_conflict", firstRunBody.DagRunId),
		api.RescheduleDAGRunJSONRequestBody{Singleton: &singleton},
	).ExpectStatus(http.StatusConflict).Send(t)

	require.Eventually(t, func() bool {
		url := fmt.Sprintf("/api/v2/dags/reschedule_singleton_conflict/dag-runs/%s", secondRunBody.DagRunId)
		statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)

		var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
		statusResp.Unmarshal(t, &dagRunStatus)
		return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
	}, 20*time.Second, 200*time.Millisecond)
}
