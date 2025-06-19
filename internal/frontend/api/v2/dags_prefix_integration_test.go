package api_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDAGWithPrefix(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("CreateListDeleteWithPrefix", func(t *testing.T) {
		// Create DAGs with various prefixes
		dags := []string{
			"root_dag",
			"workflow/task1",
			"workflow/task2",
			"workflow/etl/extract",
			"monitoring/health_check",
		}

		// Create all DAGs
		for _, dag := range dags {
			_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
				Name: dag,
			}).ExpectStatus(http.StatusCreated).Send(t)
		}

		// Test listing with prefix filter
		t.Run("ListWithWorkflowPrefix", func(t *testing.T) {
			resp := server.Client().Get("/api/v2/dags?prefix=workflow").ExpectStatus(http.StatusOK).Send(t)
			var apiResp api.ListDAGs200JSONResponse
			resp.Unmarshal(t, &apiResp)

			require.Len(t, apiResp.Dags, 2, "expected two DAGs in workflow directory")
			assert.Contains(t, apiResp.Subdirectories, "etl", "expected etl subdirectory")

			// Verify DAG names
			dagNames := make([]string, len(apiResp.Dags))
			for i, dag := range apiResp.Dags {
				dagNames[i] = dag.FileName
			}
			assert.Contains(t, dagNames, "workflow/task1")
			assert.Contains(t, dagNames, "workflow/task2")
		})

		t.Run("ListWithNestedPrefix", func(t *testing.T) {
			resp := server.Client().Get("/api/v2/dags?prefix=workflow/etl").ExpectStatus(http.StatusOK).Send(t)
			var apiResp api.ListDAGs200JSONResponse
			resp.Unmarshal(t, &apiResp)

			require.Len(t, apiResp.Dags, 1, "expected one DAG in workflow/etl directory")
			assert.Equal(t, "workflow/etl/extract", apiResp.Dags[0].FileName)
			assert.Empty(t, apiResp.Subdirectories, "expected no subdirectories")
		})

		t.Run("ListRootLevel", func(t *testing.T) {
			resp := server.Client().Get("/api/v2/dags").ExpectStatus(http.StatusOK).Send(t)
			var apiResp api.ListDAGs200JSONResponse
			resp.Unmarshal(t, &apiResp)

			// Should only show root level DAG
			var rootDAGs []string
			for _, dag := range apiResp.Dags {
				if dag.FileName == "root_dag" {
					rootDAGs = append(rootDAGs, dag.FileName)
				}
			}
			assert.Contains(t, rootDAGs, "root_dag", "expected root_dag at root level")
		})

		// Execute a prefixed DAG
		t.Run("ExecutePrefixedDAG", func(t *testing.T) {
			resp := server.Client().Post("/api/v2/dags/workflow%2Ftask1/start", api.ExecuteDAGJSONRequestBody{}).
				ExpectStatus(http.StatusOK).Send(t)

			var execResp api.ExecuteDAG200JSONResponse
			resp.Unmarshal(t, &execResp)

			require.NotEmpty(t, execResp.DagRunId, "expected a non-empty dag-run ID")

			// Check the status
			require.Eventually(t, func() bool {
				url := fmt.Sprintf("/api/v2/dags/workflow%%2Ftask1/dag-runs/%s", execResp.DagRunId)
				statusResp := server.Client().Get(url).ExpectStatus(http.StatusOK).Send(t)
				var status api.GetDAGDAGRunDetails200JSONResponse
				statusResp.Unmarshal(t, &status)

				return status.DagRun.Status == api.Status(scheduler.StatusSuccess)
			}, 5*time.Second, 1*time.Second, "expected DAG to complete")
		})

		// Delete all created DAGs
		for _, dag := range dags {
			// URL encode the DAG name for the path
			encodedName := dag
			if dag == "workflow/task1" {
				encodedName = "workflow%2Ftask1"
			} else if dag == "workflow/task2" {
				encodedName = "workflow%2Ftask2"
			} else if dag == "workflow/etl/extract" {
				encodedName = "workflow%2Fetl%2Fextract"
			} else if dag == "monitoring/health_check" {
				encodedName = "monitoring%2Fhealth_check"
			}

			_ = server.Client().Delete(fmt.Sprintf("/api/v2/dags/%s", encodedName)).
				ExpectStatus(http.StatusNoContent).Send(t)
		}
	})
}

func TestRenameDAGWithPrefix(t *testing.T) {
	server := test.SetupServer(t)

	// Create a DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "old/location/dag",
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Rename the DAG to a different prefix
	_ = server.Client().Post("/api/v2/dags/old%2Flocation%2Fdag/rename", api.RenameDAGJSONRequestBody{
		NewFileName: "new/location/dag",
	}).ExpectStatus(http.StatusOK).Send(t)

	// Verify the old DAG doesn't exist
	_ = server.Client().Get("/api/v2/dags/old%2Flocation%2Fdag").
		ExpectStatus(http.StatusNotFound).Send(t)

	// Verify the new DAG exists
	resp := server.Client().Get("/api/v2/dags/new%2Flocation%2Fdag").
		ExpectStatus(http.StatusOK).Send(t)

	var dagResp api.GetDAGDetails200JSONResponse
	resp.Unmarshal(t, &dagResp)
	assert.NotNil(t, dagResp.Dag)

	// Clean up
	_ = server.Client().Delete("/api/v2/dags/new%2Flocation%2Fdag").
		ExpectStatus(http.StatusNoContent).Send(t)
}
