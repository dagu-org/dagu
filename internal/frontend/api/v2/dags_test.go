package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
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

		require.NotEmpty(t, execResp.WorkflowId, "expected a non-empty DAG-run ID")

		// Check the status of the DAG-run
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/test_dag/workflows/%s", execResp.WorkflowId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var status api.GetDAGWorkflowDetails200JSONResponse
			statusResp.Unmarshal(t, &status)

			return status.Workflow.Status == api.Status(scheduler.StatusSuccess)
		}, 5*time.Second, 1*time.Second, "expected DAG to complete")

		// Delete the created DAG
		_ = server.Client().Delete("/api/v2/dags/test_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})
}
