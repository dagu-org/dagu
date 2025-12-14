package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

// TestAuth_BasicAuth tests that basic auth works
func TestAuth_BasicAuth(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With wrong credentials - should fail
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "wrong").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// With correct credentials - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_APIToken tests that API token auth works
func TestAuth_APIToken(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Token.Value = "my-secret-token"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With wrong token - should fail
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("wrong-token").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// With correct token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-secret-token").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BasicAuthAndAPIToken tests that both basic auth and API token work simultaneously
func TestAuth_BasicAuthAndAPIToken(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
		cfg.Server.Auth.Token.Value = "my-api-token"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With basic auth - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)

	// With API token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BuiltinMode tests that builtin auth mode works with JWT login
func TestAuth_BuiltinMode(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// Login to get JWT token
	loginResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	loginResp.Unmarshal(t, &loginResult)
	require.NotEmpty(t, loginResult.Token, "expected JWT token in response")

	// With JWT token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken(loginResult.Token).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BuiltinModeWithAPIToken tests that builtin mode works with both JWT and API token
// This is the key test for issue #1478 - API token should work alongside builtin auth
func TestAuth_BuiltinModeWithAPIToken(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Also configure API token
		cfg.Server.Auth.Token.Value = "my-api-token"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With API token - should succeed (this is the key test for issue #1478)
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusOK).Send(t)

	// Login to get JWT token
	loginResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	loginResp.Unmarshal(t, &loginResult)

	// With JWT token - should also succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken(loginResult.Token).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BuiltinModeWithBasicAuth tests that builtin mode works with both JWT and basic auth
func TestAuth_BuiltinModeWithBasicAuth(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Also configure basic auth
		cfg.Server.Auth.Basic.Username = "basicuser"
		cfg.Server.Auth.Basic.Password = "basicpass"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With basic auth - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("basicuser", "basicpass").
		ExpectStatus(http.StatusOK).Send(t)

	// Login to get JWT token
	loginResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	loginResp.Unmarshal(t, &loginResult)

	// With JWT token - should also succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken(loginResult.Token).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BuiltinModeAllMethods tests that all auth methods work simultaneously in builtin mode
func TestAuth_BuiltinModeAllMethods(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Configure all auth methods
		cfg.Server.Auth.Token.Value = "my-api-token"
		cfg.Server.Auth.Basic.Username = "basicuser"
		cfg.Server.Auth.Basic.Password = "basicpass"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With API token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusOK).Send(t)

	// With basic auth - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("basicuser", "basicpass").
		ExpectStatus(http.StatusOK).Send(t)

	// Login to get JWT token
	loginResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	loginResp.Unmarshal(t, &loginResult)

	// With JWT token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken(loginResult.Token).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_PublicPaths tests that public paths bypass authentication
func TestAuth_PublicPaths(t *testing.T) {
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
	}))

	// Health endpoint should work without auth
	server.Client().Get("/api/v2/health").ExpectStatus(http.StatusOK).Send(t)

	// Login endpoint should work without auth
	server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	// But other endpoints should require auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}
