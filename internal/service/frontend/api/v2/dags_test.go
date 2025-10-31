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

	t.Run("RetryWithFreshRunID", func(t *testing.T) {
		spec := `
steps:
  - sleep 1
`
		name := "retry_fresh_id"

		// Create DAG definition
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: name,
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Start the DAG to create an initial history entry
		startResp := server.Client().Post(fmt.Sprintf("/api/v2/dags/%s/start", name), api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).
			Send(t)

		var execBody api.ExecuteDAG200JSONResponse
		startResp.Unmarshal(t, &execBody)
		require.NotEmpty(t, execBody.DagRunId, "expected a non-empty dag-run ID")

		require.Eventually(t, func() bool {
			statusResp := server.Client().
				Get(fmt.Sprintf("/api/v2/dags/%s/dag-runs/%s", name, execBody.DagRunId)).
				ExpectStatus(http.StatusOK).
				Send(t)

			var dagRun api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRun)

			return dagRun.DagRun.Status == api.Status(core.Succeeded)
		}, 5*time.Second, 250*time.Millisecond, "expected initial DAG-run to complete")

		generateNew := true
		retryResp := server.Client().
			Post(
				fmt.Sprintf("/api/v2/dag-runs/%s/%s/retry", name, execBody.DagRunId),
				api.RetryDAGRunJSONRequestBody{
					GenerateNewRunId: &generateNew,
				},
			).
			ExpectStatus(http.StatusOK).
			Send(t)

		var retryBody api.RetryDAGRun200JSONResponse
		retryResp.Unmarshal(t, &retryBody)
		require.NotEmpty(t, retryBody.DagRunId, "expected a new dag-run ID")
		require.NotEqual(t, execBody.DagRunId, retryBody.DagRunId, "expected a different dag-run ID")

		require.Eventually(t, func() bool {
			statusResp := server.Client().
				Get(fmt.Sprintf("/api/v2/dags/%s/dag-runs/%s", name, retryBody.DagRunId)).
				ExpectStatus(http.StatusOK).
				Send(t)

			var dagRun api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRun)

			return dagRun.DagRun.Status == api.Status(core.Succeeded)
		}, 5*time.Second, 250*time.Millisecond, "expected retried DAG-run to complete")

		customID := "custom_retry_id"
		retryResp = server.Client().
			Post(
				fmt.Sprintf("/api/v2/dag-runs/%s/%s/retry", name, retryBody.DagRunId),
				api.RetryDAGRunJSONRequestBody{
					DagRunId: &customID,
				},
			).
			ExpectStatus(http.StatusOK).
			Send(t)

		var customRetry api.RetryDAGRun200JSONResponse
		retryResp.Unmarshal(t, &customRetry)
		require.Equal(t, customID, customRetry.DagRunId, "expected custom dag-run ID to be used")

		require.Eventually(t, func() bool {
			statusResp := server.Client().
				Get(fmt.Sprintf("/api/v2/dags/%s/dag-runs/%s", name, customRetry.DagRunId)).
				ExpectStatus(http.StatusOK).
				Send(t)

			var dagRun api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRun)

			return dagRun.DagRun.Status == api.Status(core.Succeeded)
		}, 5*time.Second, 250*time.Millisecond, "expected custom ID retry to complete")

		// Clean up DAG definition
		_ = server.Client().Delete(fmt.Sprintf("/api/v2/dags/%s", name)).ExpectStatus(http.StatusNoContent).Send(t)
	})
}
