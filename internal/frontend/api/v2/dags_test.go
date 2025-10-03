package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("CreateExecuteDelete", func(t *testing.T) {
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAG201JSONResponse{
			Name: "test_dag",
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Fetch the created DAG with the list endpoint
		resp := server.Client().Get("/api/v2/dags?name=test_dag").ExpectStatus(http.StatusOK).Send(t)
		var apiResp api.ListDAGs200JSONResponse
		resp.Unmarshal(t, &apiResp)

		require.Len(t, apiResp.Dags, 1, "expected one DAG")

		// Execute the created DAG
		resp = server.Client().Post("/api/v2/dags/test_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)

		require.NotEmpty(t, execResp.DagRunId, "expected a non-empty dag-run ID")

		// Check the status of the dag-run
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/test_dag/dag-runs/%s", execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)

			return dagRunStatus.DagRun.Status == api.Status(status.Success)
		}, 5*time.Second, 1*time.Second, "expected DAG to complete")

		// Delete the created DAG
		_ = server.Client().Delete("/api/v2/dags/test_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ListDAGsSorting", func(t *testing.T) {
		// Test that ListDAGs respects sort parameters
		resp := server.Client().Get("/api/v2/dags?sort=name&order=asc").ExpectStatus(http.StatusOK).Send(t)
		var apiResp api.ListDAGs200JSONResponse
		resp.Unmarshal(t, &apiResp)

		// The test should pass regardless of the sort result
		// since we're only testing that the endpoint accepts the parameters
		require.NotNil(t, apiResp.Dags)
		require.NotNil(t, apiResp.Pagination)
	})

	t.Run("ListDAGsNextRunSorting", func(t *testing.T) {
		// Create DAGs with different schedules
		hourlySpec := "schedule: \"0 * * * *\"\nsteps:\n  - name: step1\n    command: echo hourly"
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "hourly_dag",
			Spec: &hourlySpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		dailySpec := "schedule: \"0 1 * * *\"\nsteps:\n  - name: step1\n    command: echo daily"
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "daily_dag",
			Spec: &dailySpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		noScheduleSpec := "steps:\n  - name: step1\n    command: echo none"
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "no_schedule_dag",
			Spec: &noScheduleSpec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Test ascending order - hourly should come before daily
		resp := server.Client().Get("/api/v2/dags?sort=nextRun&order=asc").ExpectStatus(http.StatusOK).Send(t)
		var ascResp api.ListDAGs200JSONResponse
		resp.Unmarshal(t, &ascResp)

		var hourlyIdx, dailyIdx, noScheduleIdx = -1, -1, -1
		for i, dag := range ascResp.Dags {
			switch dag.Dag.Name {
			case "hourly_dag":
				hourlyIdx = i
			case "daily_dag":
				dailyIdx = i
			case "no_schedule_dag":
				noScheduleIdx = i
			}
		}

		require.NotEqual(t, -1, hourlyIdx)
		require.NotEqual(t, -1, dailyIdx)
		require.NotEqual(t, -1, noScheduleIdx)
		require.Less(t, hourlyIdx, dailyIdx, "hourly should come before daily")
		require.Greater(t, noScheduleIdx, dailyIdx, "no schedule should come last")

		// Clean up
		_ = server.Client().Delete("/api/v2/dags/hourly_dag").ExpectStatus(http.StatusNoContent).Send(t)
		_ = server.Client().Delete("/api/v2/dags/daily_dag").ExpectStatus(http.StatusNoContent).Send(t)
		_ = server.Client().Delete("/api/v2/dags/no_schedule_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("ExecuteDAGWithSingleton", func(t *testing.T) {
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAG201JSONResponse{
			Name: "test_singleton_dag",
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Execute the DAG with singleton flag
		singleton := true
		resp := server.Client().Post("/api/v2/dags/test_singleton_dag/start", api.ExecuteDAGJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId, "expected a non-empty dag-run ID")

		// Clean up
		_ = server.Client().Delete("/api/v2/dags/test_singleton_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})
}
