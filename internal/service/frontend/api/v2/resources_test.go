package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetResourceHistory(t *testing.T) {
	t.Parallel()

	// Note: SetupServer passes nil for resourceService, so this tests the error path
	server := test.SetupServer(t)

	resp := server.Client().Get("/api/v2/services/resources/history").
		ExpectStatus(http.StatusInternalServerError).
		Send(t)

	var errResp api.Error
	resp.Unmarshal(t, &errResp)

	assert.Equal(t, api.ErrorCodeInternalError, errResp.Code)
	assert.Contains(t, errResp.Message, "Resource service not available")
}

func TestGetResourceHistory_WithDuration(t *testing.T) {
	t.Parallel()

	server := test.SetupServer(t)

	// Test with duration parameter - still returns error since no resource service
	resp := server.Client().Get("/api/v2/services/resources/history?duration=30m").
		ExpectStatus(http.StatusInternalServerError).
		Send(t)

	var errResp api.Error
	resp.Unmarshal(t, &errResp)

	require.Equal(t, api.ErrorCodeInternalError, errResp.Code)
}
