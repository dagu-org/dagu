package api_test

import (
	"net/http"
	"testing"

	api "github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestHealthCheck(t *testing.T) {
	server := test.SetupServer(t)
	resp := server.Client().Get("/api/v1/health").ExpectStatus(http.StatusOK).Send(t)

	var healthResp api.HealthResponse
	resp.Unmarshal(t, &healthResp)

	require.Equal(t, api.HealthResponseStatusHealthy, healthResp.Status, "expected status 'ok'")
}

func TestHealthCheck_BypassesAuth(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	resp := server.Client().Get("/api/v1/health").ExpectStatus(http.StatusOK).Send(t)

	var healthResp api.HealthResponse
	resp.Unmarshal(t, &healthResp)
	require.Equal(t, api.HealthResponseStatusHealthy, healthResp.Status)

	server.Client().Get("/api/v1/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}
