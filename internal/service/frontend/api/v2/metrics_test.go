package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestMetrics_PublicMode(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPublic
	}))

	// Should work without auth when public
	resp := server.Client().Get("/api/v2/metrics").ExpectStatus(http.StatusOK).Send(t)
	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)

	// Other endpoints still require auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}

func TestMetrics_PrivateMode_Unauthorized(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPrivate
	}))

	// Should require auth when private
	server.Client().Get("/api/v2/metrics").ExpectStatus(http.StatusUnauthorized).Send(t)
}

func TestMetrics_PrivateMode_WithBasicAuth(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Metrics = config.MetricsAccessPrivate
	}))

	// Should work with valid basic auth
	resp := server.Client().Get("/api/v2/metrics").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)
	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)
}

func TestMetrics_PrivateMode_WithAPIToken(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Token.Value = "test-api-token"
		cfg.Server.Metrics = config.MetricsAccessPrivate
	}))

	// Should work with valid API token
	resp := server.Client().Get("/api/v2/metrics").
		WithBearerToken("test-api-token").
		ExpectStatus(http.StatusOK).Send(t)
	require.Contains(t, resp.Response.Header().Get("Content-Type"), "text/plain")
	require.NotEmpty(t, resp.Body)
}

func TestMetrics_DefaultsToPrivate(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		// Don't set Metrics - should default to private
	}))

	// Should require auth by default
	server.Client().Get("/api/v2/metrics").ExpectStatus(http.StatusUnauthorized).Send(t)
}
