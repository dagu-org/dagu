package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestAuth_Combinations(t *testing.T) {
	t.Parallel()

	type authConfig struct {
		mode      config.AuthMode
		basicUser string
		basicPass string
	}

	type request struct {
		name       string
		basicUser  string
		basicPass  string
		wantStatus int
	}

	tests := []struct {
		name     string
		config   authConfig
		requests []request
	}{
		{
			name:   "default_no_auth",
			config: authConfig{},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "random_basic", basicUser: "user", basicPass: "pass", wantStatus: http.StatusOK},
			},
		},
		{
			name:   "mode_none_no_auth",
			config: authConfig{mode: config.AuthModeNone},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "random_basic", basicUser: "user", basicPass: "pass", wantStatus: http.StatusOK},
			},
		},

		{
			name:   "default_basic",
			config: authConfig{mode: config.AuthModeBasic, basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_user", basicUser: "wrong", basicPass: "secret", wantStatus: http.StatusUnauthorized},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		{
			name:   "mode_none_basic",
			config: authConfig{mode: config.AuthModeNone, basicUser: "admin", basicPass: "secret"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "valid_basic", basicUser: "admin", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_user", basicUser: "wrong", basicPass: "secret", wantStatus: http.StatusOK},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusOK},
			},
		},

		{
			name:   "default_basic_special_chars",
			config: authConfig{mode: config.AuthModeBasic, basicUser: "admin", basicPass: "p@ss$word&with`special"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusUnauthorized},
				{name: "valid_basic", basicUser: "admin", basicPass: "p@ss$word&with`special", wantStatus: http.StatusOK},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusUnauthorized},
			},
		},
		{
			name:   "mode_none_basic_special_chars",
			config: authConfig{mode: config.AuthModeNone, basicUser: "admin", basicPass: "p@ss$word&with`special"},
			requests: []request{
				{name: "no_creds", wantStatus: http.StatusOK},
				{name: "valid_basic", basicUser: "admin", basicPass: "p@ss$word&with`special", wantStatus: http.StatusOK},
				{name: "wrong_pass", basicUser: "admin", basicPass: "wrong", wantStatus: http.StatusOK},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
				cfg.Server.Auth.Mode = tt.config.mode
				cfg.Server.Auth.Basic.Username = tt.config.basicUser
				cfg.Server.Auth.Basic.Password = tt.config.basicPass
			}))

			for _, req := range tt.requests {
				t.Run(req.name, func(t *testing.T) {
					client := server.Client().Get("/api/v1/dag-runs")
					if req.basicUser != "" {
						client = client.WithBasicAuth(req.basicUser, req.basicPass)
					}
					client.ExpectStatus(req.wantStatus).Send(t)
				})
			}
		})
	}
}

func TestAuth_BuiltinMode(t *testing.T) {
	t.Parallel()

	t.Run("jwt_only", func(t *testing.T) {
		t.Parallel()

		server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
			cfg.Server.Auth.Mode = config.AuthModeBuiltin
			cfg.Server.Auth.Builtin.Admin.Username = "admin"
			cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
			cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
			cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		}))

		// Without auth - should fail
		server.Client().Get("/api/v1/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)

		// Login to get JWT token
		loginResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "admin",
			Password: "adminpass",
		}).ExpectStatus(http.StatusOK).Send(t)

		var loginResult api.LoginResponse
		loginResp.Unmarshal(t, &loginResult)
		require.NotEmpty(t, loginResult.Token)

		// With JWT token - should succeed
		server.Client().Get("/api/v1/dag-runs").
			WithBearerToken(loginResult.Token).
			ExpectStatus(http.StatusOK).Send(t)
	})

	// Basic auth is no longer available alongside builtin mode.
	// Under the new auth model, basic auth is only available when
	// auth.mode is explicitly set to "basic". Setting basic credentials
	// under builtin mode is a configuration error.
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
	server.Client().Get("/api/v1/health").ExpectStatus(http.StatusOK).Send(t)

	// Login endpoint - public
	server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	// Other endpoints - require auth
	server.Client().Get("/api/v1/dag-runs").ExpectStatus(http.StatusUnauthorized).Send(t)
}
