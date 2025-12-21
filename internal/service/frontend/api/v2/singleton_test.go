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

func TestSingleton(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("ExecuteDAGConflict", func(t *testing.T) {
		spec := `
steps:
  - name: sleep
    command: sleep 10
`
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "singleton_exec_dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Execute the DAG
		singleton := true
		resp := server.Client().Post("/api/v2/dags/singleton_exec_dag/start", api.ExecuteDAGJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)
		require.NotEmpty(t, execResp.DagRunId)

		// Wait for it to be running
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/singleton_exec_dag/dag-runs/%s", execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Running)
		}, 5*time.Second, 100*time.Millisecond)

		// Try to execute it again with singleton: true - should conflict
		server.Client().Post("/api/v2/dags/singleton_exec_dag/start", api.ExecuteDAGJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusConflict).Send(t)

		// Clean up (deleting the DAG will eventually stop the run)
		_ = server.Client().Delete("/api/v2/dags/singleton_exec_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("EnqueueDAGConflict_Running", func(t *testing.T) {
		spec := `
steps:
  - name: sleep
    command: sleep 10
`
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "singleton_enq_run_dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Execute the DAG
		resp := server.Client().Post("/api/v2/dags/singleton_enq_run_dag/start", api.ExecuteDAGJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var execResp api.ExecuteDAG200JSONResponse
		resp.Unmarshal(t, &execResp)

		// Wait for it to be running
		require.Eventually(t, func() bool {
			url := fmt.Sprintf("/api/v2/dags/singleton_enq_run_dag/dag-runs/%s", execResp.DagRunId)
			statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
			var dagRunStatus api.GetDAGDAGRunDetails200JSONResponse
			statusResp.Unmarshal(t, &dagRunStatus)
			return dagRunStatus.DagRun.Status == api.Status(core.Running)
		}, 5*time.Second, 100*time.Millisecond)

		// Try to enqueue it with singleton: true - should conflict because it's running
		singleton := true
		server.Client().Post("/api/v2/dags/singleton_enq_run_dag/enqueue", api.EnqueueDAGDAGRunJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusConflict).Send(t)

		// Clean up
		_ = server.Client().Delete("/api/v2/dags/singleton_enq_run_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("EnqueueDAGConflict_Queued", func(t *testing.T) {
		spec := `
steps:
  - name: sleep
    command: sleep 10
`
		// Create a new DAG
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "singleton_enq_q_dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Enqueue the DAG
		resp := server.Client().Post("/api/v2/dags/singleton_enq_q_dag/enqueue", api.EnqueueDAGDAGRunJSONRequestBody{}).
			ExpectStatus(http.StatusOK).Send(t)

		var enqueueResp api.EnqueueDAGDAGRun200JSONResponse
		resp.Unmarshal(t, &enqueueResp)

		// Try to enqueue it again with singleton: true - should conflict because it's already queued
		singleton := true
		server.Client().Post("/api/v2/dags/singleton_enq_q_dag/enqueue", api.EnqueueDAGDAGRunJSONRequestBody{
			Singleton: &singleton,
		}).ExpectStatus(http.StatusConflict).Send(t)

		// Clean up
		_ = server.Client().Delete("/api/v2/dags/singleton_enq_q_dag").ExpectStatus(http.StatusNoContent).Send(t)
	})
}
