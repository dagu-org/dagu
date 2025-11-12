package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestMetrics_BypassesAuth(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	resp := server.Client().Get("/api/v2/metrics").ExpectStatus(http.StatusOK).Send(t)

	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)

	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}
