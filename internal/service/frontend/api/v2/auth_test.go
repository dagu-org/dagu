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

// TestAuth_Combinations tests all auth mode and configuration combinations
// in a table-driven format for easy verification of correctness.
func TestAuth_Combinations(t *testing.T) {
	t.Parallel()

	type authConfig struct {
		mode       config.AuthMode
		token      string
		basicUser  string
		basicPass  string
	}

	type request struct {
		name       string
		token      string // bearer token to send
		basicUser  string // basic auth username
		basicPass  string // basic auth password
		wantStatus int
	}

	tests := []struct {
		name     string
		config   authConfig
		requests []request
	}{
		// ===========================================
		// No auth configured (mode not set)
		// ===========================================
		{
			name:   "default_no_auth",
			config: authConfig{},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "random_token", token: "random", wantStatus: http.StatusOK},
				{name: "random_basic", basicUser: "user", basicPass: "pass", wantStatus: http.StatusOK},
			},
		},
		// ===========================================
		// No auth configured (mode=none)
		// ===========================================
		{
			name:   "mode_none_no_auth",
			config: authConfig{mode: config.AuthModeNone},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "random_token", token: "random", wantStatus: http.StatusOK},
				{name: "random_basic", basicUser: "user", basicPass: "pass", wantStatus: http.StatusOK},
			},
		},

		// ===========================================
		// Token only (mode not set)
		// ===========================================
		{
			name:   "default_token",
			config: authConfig{token: "secret-token"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_token", token: "secret-token", wantStatus: http.StatusOK},
				{name: "wrong_token", token: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		// ===========================================
		// Token only (mode=none)
		// ===========================================
		{
			name:   "mode_none_token",
			config: authConfig{mode: config.AuthModeNone, token: "secret-token"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_token", token: "secret-token", wantStatus: http.StatusOK},
				{name: "wrong_token", token: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},

		// ===========================================
		// Basic auth only (mode not set)
		// ===========================================
		{
			name:   "default_basic",
			config: authConfig{basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_user", basicUser: "wrong", basicPass: "secret", wantStatus: http.StatusUnauthorized},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		// ===========================================
		// Basic auth only (mode=none)
		// ===========================================
		{
			name:   "mode_none_basic",
			config: authConfig{mode: config.AuthModeNone, basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_user", basicUser: "wrong", basicPass: "secret", wantStatus: http.StatusUnauthorized},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},

		// ===========================================
		// Basic auth with special chars (mode not set)
		// ===========================================
		{
			name:   "default_basic_special_chars",
			config: authConfig{basicUser: "admin", basicPass: "p@ss$word&with`special"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "p@ss$word&with`special", wantStatus: http.StatusOK},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		// ===========================================
		// Basic auth with special chars (mode=none)
		// ===========================================
		{
			name:   "mode_none_basic_special_chars",
			config: authConfig{mode: config.AuthModeNone, basicUser: "admin", basicPass: "p@ss$word&with`special"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "p@ss$word&with`special", wantStatus: http.StatusOK},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},

		// ===========================================
		// Both token and basic auth (mode not set)
		// ===========================================
		{
			name:   "default_token_and_basic",
			config: authConfig{token: "my-token", basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_token", token: "my-token", wantStatus: http.StatusOK},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_token", token: "wrong", wantStatus: http.StatusUnauthorized},
				{name: "wrong_basic", basicUser: "wrong", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		// ===========================================
		// Both token and basic auth (mode=none)
		// ===========================================
		{
			name:   "mode_none_token_and_basic",
			config: authConfig{mode: config.AuthModeNone, token: "my-token", basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_token", token: "my-token", wantStatus: http.StatusOK},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_token", token: "wrong", wantStatus: http.StatusUnauthorized},
				{name: "wrong_basic", basicUser: "wrong", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
				cfg.Server.Auth.Mode = tt.config.mode
				cfg.Server.Auth.Token.Value = tt.config.token
				cfg.Server.Auth.Basic.Username = tt.config.basicUser
				cfg.Server.Auth.Basic.Password = tt.config.basicPass
			}))

			for _, req := range tt.requests {
				t.Run(req.name, func(t *testing.T) {
					client := server.Client().Get("/api/v2/dag-runs")
					if req.token != "" {
						client = client.WithBearerToken(req.token)
					}
					if req.basicUser != "" {
						client = client.WithBasicAuth(req.basicUser, req.basicPass)
					}
					client.ExpectStatus(req.wantStatus).Send(t)
				})
			}
		})
	}
}

// TestAuth_BuiltinMode tests builtin auth mode with JWT login
func TestAuth_BuiltinMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		token      string // API token (not JWT)
		basicUser  string
		basicPass  string
	}{
		{name: "jwt_only"},
		{name: "jwt_with_token", token: "api-token"},
		{name: "jwt_with_basic", basicUser: "basicuser", basicPass: "basicpass"},
		{name: "jwt_with_token_and_basic", token: "api-token", basicUser: "basicuser", basicPass: "basicpass"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
				cfg.Server.Auth.Mode = config.AuthModeBuiltin
				cfg.Server.Auth.Builtin.Admin.Username = "admin"
				cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
				cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
				cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
				cfg.Server.Auth.Token.Value = tt.token
				cfg.Server.Auth.Basic.Username = tt.basicUser
				cfg.Server.Auth.Basic.Password = tt.basicPass
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
			require.NotEmpty(t, loginResult.Token)

			// With JWT token - should succeed
			server.Client().Get("/api/v2/dag-runs").
				WithBearerToken(loginResult.Token).
				ExpectStatus(http.StatusOK).Send(t)

			// With API token (if configured) - should succeed
			if tt.token != "" {
				server.Client().Get("/api/v2/dag-runs").
					WithBearerToken(tt.token).
					ExpectStatus(http.StatusOK).Send(t)

				// Wrong API token - should fail
				server.Client().Get("/api/v2/dag-runs").
					WithBearerToken("wrong-token").
					ExpectStatus(http.StatusUnauthorized).Send(t)
			}

			// With basic auth (if configured) - should succeed
			if tt.basicUser != "" {
				server.Client().Get("/api/v2/dag-runs").
					WithBasicAuth(tt.basicUser, tt.basicPass).
					ExpectStatus(http.StatusOK).Send(t)

				// Wrong basic auth - should fail
				server.Client().Get("/api/v2/dag-runs").
					WithBasicAuth("wrong", "wrong").
					ExpectStatus(http.StatusUnauthorized).Send(t)
			}
		})
	}
}

// TestAuth_BuiltinModeWriteExecute tests that API token can perform
// write and execute operations in builtin mode
func TestAuth_BuiltinModeWriteExecute(t *testing.T) {
	t.Parallel()

	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
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

	// Create DAG with API token
	server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: "api_token_test_dag",
		Spec: &spec,
	}).WithBearerToken("my-api-token").ExpectStatus(http.StatusCreated).Send(t)

	// Start DAG with API token
	server.Client().Post("/api/v2/dags/api_token_test_dag/start", api.ExecuteDAGJSONRequestBody{}).
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusOK).Send(t)

	// Delete DAG with API token
	server.Client().Delete("/api/v2/dags/api_token_test_dag").
		WithBearerToken("my-api-token").
		ExpectStatus(http.StatusNoContent).Send(t)
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

	// Health endpoint - public
	server.Client().Get("/api/v2/health").ExpectStatus(http.StatusOK).Send(t)

	// Login endpoint - public
	server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	// Other endpoints - require auth
	server.Client().Get("/api/v2/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}
