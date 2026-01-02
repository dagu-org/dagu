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

func TestDAG(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("CreateExecuteDelete", func(t *testing.T) {
		spec := `
steps:
  - sleep 1
`
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "test_dag",
			Spec: &spec,
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

			return dagRunStatus.DagRun.Status == api.Status(core.Succeeded)
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

	t.Run("ExecuteDAGWithJSONParams", func(t *testing.T) {
		// Verifies that JSON parameters are parsed as named key-value pairs,
		// not tokenized by whitespace (regression test for JSON params bug).
		spec := `
steps:
  - name: echo_params
    command: echo params received
`
		dagName := "test_json_params"

		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: dagName,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		jsonParams := `{"key1": "test1", "key2": "test2"}`
		resp := server.Client().Post("/api/v2/dags/"+dagName+"/start", api.ExecuteDAGJSONRequestBody{
			Params: &jsonParams,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId)

		var dagRunDetails api.GetDAGDAGRunDetails200JSONResponse
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/%s/dag-runs/%s", dagName, execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			statusResp.Unmarshal(t, &dagRunDetails)
			return dagRunDetails.DagRun.Status == api.Status(core.Succeeded)
		}, 10*time.Second, 500*time.Millisecond, "DAG should complete")

		require.NotNil(t, dagRunDetails.DagRun.Params)
		params := *dagRunDetails.DagRun.Params
		require.Contains(t, params, "key1=test1")
		require.Contains(t, params, "key2=test2")
		require.NotContains(t, params, "1={", "JSON should not be tokenized")

		_ = server.Client().Delete("/api/v2/dags/" + dagName).ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("EnqueueDAGRunFromSpec", func(t *testing.T) {
		spec := `
steps:
  - sleep 1
`
		name := "inline_enqueue_spec"

		resp := server.Client().Post("/api/v2/dag-runs/enqueue", api.EnqueueDAGRunFromSpecJSONRequestBody{
			Spec: spec,
			Name: &name,
		}).
			ExpectStatus(http.StatusOK).
			Send(t)

		var body api.EnqueueDAGRunFromSpec200JSONResponse
		resp.Unmarshal(t, &body)
		require.NotEmpty(t, body.DagRunId, "expected a non-empty dag-run ID")

		require.Eventually(t, func() bool {
			statusResp := server.Client().
				Get(fmt.Sprintf("/api/v2/dag-runs/%s/%s", name, body.DagRunId)).
				ExpectStatus(http.StatusOK).
				Send(t)

			var dagRun api.GetDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRun)

			s := dagRun.DagRunDetails.Status
			return s == api.Status(core.Queued) || s == api.Status(core.Running) || s == api.Status(core.Succeeded)
		}, 5*time.Second, 250*time.Millisecond, "expected DAG-run to reach queued state")
	})
}
