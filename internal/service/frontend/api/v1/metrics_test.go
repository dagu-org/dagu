package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestMetrics_PublicMode(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPublic
	}))

	resp := server.Client().Get("/api/v1/metrics").ExpectStatus(http.StatusOK).Send(t)
	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)

	server.Client().Get("/api/v1/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}

func TestMetrics_PrivateMode_Unauthorized(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPrivate
	}))

	server.Client().Get("/api/v1/metrics").ExpectStatus(http.StatusUnauthorized).Send(t)
}

func TestMetrics_PrivateMode_WithBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPrivate
	}))

	resp := server.Client().Get("/api/v1/metrics").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)
	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)
}

func TestMetrics_DefaultsToPrivate(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	server.Client().Get("/api/v1/metrics").ExpectStatus(http.StatusUnauthorized).Send(t)
}
