package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestGetWorkers(t *testing.T) {
	t.Run("NoCoordinatorConfigured", func(t *testing.T) {
		// Set up a server without coordinator configuration
		server := test.SetupServer(t)

		// Make the request
		resp := server.Client().Get("/api/v1/workers").ExpectStatus(http.StatusOK).Send(t)

		var workersResp api.WorkersListResponse
		resp.Unmarshal(t, &workersResp)

		// Should return empty workers list when no coordinators are available
		require.Empty(t, workersResp.Workers)
		require.Empty(t, workersResp.Errors)
	})

	// Additional integration tests would require a real coordinator running
	// For now, the unit test approach with mocks would be better
	// but that would require refactoring the test setup
}
