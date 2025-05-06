package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	server := test.SetupServer(t)

	// Create a new DAG
	_ = server.Client().Post("/api/v2/dags", api.CreateNewDAG201JSONResponse{
		Name: "test_dag",
	}).ExpectStatus(http.StatusCreated).Send(t)

	// Fetch the created DAG with the list endpoint
	resp := server.Client().Get("/api/v2/dags?name=test_dag").ExpectStatus(http.StatusOK).Send(t)
	var apiResp api.ListDAGs200JSONResponse
	resp.Unmarshal(t, &apiResp)

	require.Len(t, apiResp.Dags, 1, "expected one DAG")

	// Delete the created DAG
	_ = server.Client().Delete("/api/v2/dags/test_dag").ExpectStatus(http.StatusNoContent).Send(t)
}
