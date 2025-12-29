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

// TestAuth_NoAuthConfigured tests that without any auth configured, requests are allowed
func TestAuth_NoAuthConfigured(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t)

	// Without any auth configured, requests should be allowed
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_BasicAuthRequired tests that with basic auth configured, requests without auth fail
func TestAuth_BasicAuthRequired(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// Without auth - should fail with 401
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestAuth_BasicAuth tests that basic auth works
func TestAuth_BasicAuth(t *testing.T) {
	t.Parallel()
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

// TestAuth_BasicAuthSpecialChars tests that passwords with special characters work
func TestAuth_BasicAuthSpecialChars(t *testing.T) {
	t.Parallel()
	// Password with special characters: $, &, @, `, etc.
	specialPassword := "p@ss$word&with`special"
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = specialPassword
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With correct credentials including special chars - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", specialPassword).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_APIToken tests that API token auth works
func TestAuth_APIToken(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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

// TestAuth_BuiltinModeWithAPIToken_WriteExecute tests that API token can perform
// write and execute operations in builtin mode (regression test for issue #1478)
func TestAuth_BuiltinModeWithAPIToken_WriteExecute(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Also configure API token
		cfg.Server.Auth.Token.Value = "my-api-token"
	}))

	spec := `
steps:
  - name: test
    command: echo hello
`

	// Without auth - write should fail
	server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "api_token_test_dag",
		Spec: &spec,
	}).ExpectStatus(http.StatusUnauthorized).Send(t)

	// Test write operation (create DAG) with API token - this was failing before the fix
	server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "api_token_test_dag",
		Spec: &spec,
	}).WithBearerToken("my-api-token").ExpectStatus(http.StatusCreated).Send(t)

	// Test execute operation (start DAG) with API token - this was also failing
	server.Client().Post("/api/v2/dags/api_token_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusOK).Send(t)

	// Test delete operation with API token
	server.Client().Delete("/api/v2/dags/api_token_test_dag").
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusNoContent).Send(t)
}

// TestAuth_BuiltinModeIgnoresBasicAuth tests that basic auth is ignored in builtin mode
func TestAuth_BuiltinModeIgnoresBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Configure basic auth (should be ignored in builtin mode)
		cfg.Server.Auth.Basic.Username = "basicuser"
		cfg.Server.Auth.Basic.Password = "basicpass"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With basic auth - should fail because basic auth is ignored in builtin mode
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("basicuser", "basicpass").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// Login to get JWT token - this should work
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
	t.Parallel()
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

// =============================================================================
// Auth Mode + Configuration Combination Tests
// =============================================================================
// These tests verify that all combinations of auth mode and auth configuration
// behave correctly. This is important because auth mode should take precedence
// over individual auth configurations.

// TestAuth_ModeNoneWithToken tests that when auth mode is "none",
// requests are allowed even if an API token is configured.
// This was a bug where token auth was enabled regardless of auth mode.
func TestAuth_ModeNoneWithToken(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeNone
		cfg.Server.Auth.Token.Value = "configured-but-should-be-ignored"
	}))

	// With mode=none, requests should succeed without any auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusOK).Send(t)

	// Token should also work (but not be required)
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("configured-but-should-be-ignored").
		ExpectStatus(http.StatusOK).Send(t)

	// Wrong token should also work because auth is disabled
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("wrong-token").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_ModeNoneWithBasicAuth tests that when auth mode is "none",
// requests are allowed even if basic auth credentials are configured.
func TestAuth_ModeNoneWithBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeNone
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// With mode=none, requests should succeed without any auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusOK).Send(t)

	// Basic auth should also work (but not be required)
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)

	// Wrong credentials should also work because auth is disabled
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("wrong", "wrong").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_ModeNoneWithTokenAndBasicAuth tests that when auth mode is "none",
// requests are allowed even if both token and basic auth are configured.
func TestAuth_ModeNoneWithTokenAndBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeNone
		cfg.Server.Auth.Token.Value = "my-token"
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// With mode=none, requests should succeed without any auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusOK).Send(t)

	// All auth methods should work but none should be required
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-token").
		ExpectStatus(http.StatusOK).Send(t)

	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)

	// Wrong credentials should also work because auth is disabled
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("wrong").
		ExpectStatus(http.StatusOK).Send(t)

	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("wrong", "wrong").
		ExpectStatus(http.StatusOK).Send(t)
}

// TestAuth_DefaultModeWithToken tests that when auth mode is not explicitly set
// (empty/default), token auth is enabled if a token is configured.
func TestAuth_DefaultModeWithToken(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		// Don't set auth mode explicitly (empty string)
		cfg.Server.Auth.Token.Value = "my-secret-token"
	}))

	// Without auth - should fail because token is configured
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With correct token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-secret-token").
		ExpectStatus(http.StatusOK).Send(t)

	// With wrong token - should fail
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("wrong-token").
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestAuth_DefaultModeWithBasicAuth tests that when auth mode is not explicitly set,
// basic auth is enabled if credentials are configured.
func TestAuth_DefaultModeWithBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		// Don't set auth mode explicitly
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With correct credentials - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)

	// With wrong credentials - should fail
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("wrong", "wrong").
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestAuth_DefaultModeWithTokenAndBasicAuth tests that when auth mode is not explicitly set,
// both token and basic auth work if both are configured.
func TestAuth_DefaultModeWithTokenAndBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		// Don't set auth mode explicitly
		cfg.Server.Auth.Token.Value = "my-token"
		cfg.Server.Auth.Basic.Username = "admin"
		cfg.Server.Auth.Basic.Password = "secret"
	}))

	// Without auth - should fail
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

	// With token - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("my-token").
		ExpectStatus(http.StatusOK).Send(t)

	// With basic auth - should succeed
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("admin", "secret").
		ExpectStatus(http.StatusOK).Send(t)

	// With wrong credentials - should fail
	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken("wrong").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("wrong", "wrong").
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestAuth_BuiltinModeWithTokenAndBasicAuth tests that in builtin mode,
// token auth works but basic auth is ignored (users should use builtin login).
func TestAuth_BuiltinModeWithTokenAndBasicAuth(t *testing.T) {
	t.Parallel()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		// Configure both token and basic auth
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

	// With basic auth - should fail (basic auth is ignored in builtin mode)
	server.Client().Get("/api/v2/dag-runs").
		WithBasicAuth("basicuser", "basicpass").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// With JWT from login - should succeed
	loginResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	loginResp.Unmarshal(t, &loginResult)

	server.Client().Get("/api/v2/dag-runs").
		WithBearerToken(loginResult.Token).
		ExpectStatus(http.StatusOK).Send(t)
}
