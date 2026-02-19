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
			cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
			cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		}))

		// Create admin via setup
		server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "admin",
			Password: "adminpass",
		}).ExpectStatus(http.StatusOK).Send(t)

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
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
	}))

	// Create admin via setup
	server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

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

// loginAndGetToken is a helper that logs in and returns the JWT token.
func loginAndGetToken(t *testing.T, server test.Server, username, password string) string {
	t.Helper()
	resp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: username,
		Password: password,
	}).ExpectStatus(http.StatusOK).Send(t)

	var result api.LoginResponse
	resp.Unmarshal(t, &result)
	require.NotEmpty(t, result.Token)
	return result.Token
}

// builtinServer creates a test server with builtin auth and creates an admin
// via the setup endpoint.
func builtinServer(t *testing.T) test.Server {
	t.Helper()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Token.Secret = "test-jwt-secret-key-integration"
		cfg.Server.Auth.Builtin.Token.TTL = time.Hour
	}))

	// Create admin via setup endpoint
	server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	return server
}

// setupServer creates a test server with builtin auth but NO admin credentials,
// so the setup page is active.
func setupServer(t *testing.T) test.Server {
	t.Helper()
	return test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Token.Secret = "test-jwt-secret-key-setup"
		cfg.Server.Auth.Builtin.Token.TTL = time.Hour
	}))
}

func TestSetup(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		server := setupServer(t)

		resp := server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "myadmin",
			Password: "securepass1",
		}).ExpectStatus(http.StatusOK).Send(t)

		var result api.LoginResponse
		resp.Unmarshal(t, &result)
		require.NotEmpty(t, result.Token)
		require.Equal(t, "myadmin", result.User.Username)
		require.Equal(t, api.UserRoleAdmin, result.User.Role)
		require.False(t, result.ExpiresAt.IsZero())
	})

	t.Run("returns_valid_token", func(t *testing.T) {
		t.Parallel()

		server := setupServer(t)

		resp := server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "myadmin",
			Password: "securepass1",
		}).ExpectStatus(http.StatusOK).Send(t)

		var result api.LoginResponse
		resp.Unmarshal(t, &result)

		// The returned token should grant access to protected endpoints
		server.Client().Get("/api/v1/dag-runs").
			WithBearerToken(result.Token).
			ExpectStatus(http.StatusOK).Send(t)
	})

	t.Run("already_completed", func(t *testing.T) {
		t.Parallel()

		server := setupServer(t)

		// First setup succeeds
		server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "myadmin",
			Password: "securepass1",
		}).ExpectStatus(http.StatusOK).Send(t)

		// Second setup fails
		server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "another",
			Password: "securepass2",
		}).ExpectStatus(http.StatusForbidden).Send(t)
	})

	t.Run("weak_password", func(t *testing.T) {
		t.Parallel()

		server := setupServer(t)

		server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "myadmin",
			Password: "short",
		}).ExpectStatus(http.StatusBadRequest).Send(t)
	})

	t.Run("setup_is_public_path", func(t *testing.T) {
		t.Parallel()

		// Setup endpoint must be accessible without any authentication
		// (no bearer token, no basic auth) since it's the first-run flow.
		server := setupServer(t)

		server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
			Username: "myadmin",
			Password: "securepass1",
		}).ExpectStatus(http.StatusOK).Send(t)
	})
}

func TestLogin(t *testing.T) {
	t.Parallel()

	server := builtinServer(t)

	t.Run("success", func(t *testing.T) {
		resp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "admin",
			Password: "adminpass",
		}).ExpectStatus(http.StatusOK).Send(t)

		var result api.LoginResponse
		resp.Unmarshal(t, &result)
		require.NotEmpty(t, result.Token)
		require.Equal(t, "admin", result.User.Username)
		require.Equal(t, api.UserRoleAdmin, result.User.Role)
		require.False(t, result.ExpiresAt.IsZero())
		require.NotEmpty(t, result.User.Id)
	})

	t.Run("invalid_password", func(t *testing.T) {
		server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "admin",
			Password: "wrongpass",
		}).ExpectStatus(http.StatusUnauthorized).Send(t)
	})

	t.Run("invalid_username", func(t *testing.T) {
		server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "nonexistent",
			Password: "adminpass",
		}).ExpectStatus(http.StatusUnauthorized).Send(t)
	})

	t.Run("empty_credentials", func(t *testing.T) {
		server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "",
			Password: "",
		}).ExpectStatus(http.StatusUnauthorized).Send(t)
	})
}

func TestLogin_DisabledUser(t *testing.T) {
	t.Parallel()

	server := setupServer(t)

	// Create admin via setup
	setupResp := server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
		Username: "admin",
		Password: "adminpass1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var setupResult api.LoginResponse
	setupResp.Unmarshal(t, &setupResult)
	adminToken := setupResult.Token

	// Create a second user
	createResp := server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "testuser",
		Password: "testpass123",
		Role:     api.UserRoleDeveloper,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.UserResponse
	createResp.Unmarshal(t, &createResult)

	// Verify the user can login
	server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "testuser",
		Password: "testpass123",
	}).ExpectStatus(http.StatusOK).Send(t)

	// Disable the user
	isDisabled := true
	server.Client().Patch("/api/v1/users/"+createResult.User.Id, api.UpdateUserRequest{
		IsDisabled: &isDisabled,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusOK).Send(t)

	// Login should now fail with 401
	server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "testuser",
		Password: "testpass123",
	}).ExpectStatus(http.StatusUnauthorized).Send(t)
}

func TestGetCurrentUser(t *testing.T) {
	t.Parallel()

	server := builtinServer(t)

	t.Run("authenticated", func(t *testing.T) {
		token := loginAndGetToken(t, server, "admin", "adminpass")

		resp := server.Client().Get("/api/v1/auth/me").
			WithBearerToken(token).
			ExpectStatus(http.StatusOK).Send(t)

		var result api.UserResponse
		resp.Unmarshal(t, &result)
		require.Equal(t, "admin", result.User.Username)
		require.Equal(t, api.UserRoleAdmin, result.User.Role)
		require.NotEmpty(t, result.User.Id)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		server.Client().Get("/api/v1/auth/me").
			ExpectStatus(http.StatusUnauthorized).Send(t)
	})

	t.Run("invalid_token", func(t *testing.T) {
		server.Client().Get("/api/v1/auth/me").
			WithBearerToken("invalid-jwt-token").
			ExpectStatus(http.StatusUnauthorized).Send(t)
	})
}

func TestChangePassword(t *testing.T) {
	t.Parallel()

	t.Run("success_and_verify", func(t *testing.T) {
		t.Parallel()

		server := builtinServer(t)
		token := loginAndGetToken(t, server, "admin", "adminpass")

		// Change password
		server.Client().Post("/api/v1/auth/change-password", api.ChangePasswordRequest{
			CurrentPassword: "adminpass",
			NewPassword:     "newpassword123",
		}).WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

		// Can login with new password
		loginAndGetToken(t, server, "admin", "newpassword123")

		// Old password no longer works
		server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: "admin",
			Password: "adminpass",
		}).ExpectStatus(http.StatusUnauthorized).Send(t)
	})

	t.Run("wrong_current_password", func(t *testing.T) {
		t.Parallel()

		server := builtinServer(t)
		token := loginAndGetToken(t, server, "admin", "adminpass")

		server.Client().Post("/api/v1/auth/change-password", api.ChangePasswordRequest{
			CurrentPassword: "wrongpassword",
			NewPassword:     "newpassword123",
		}).WithBearerToken(token).ExpectStatus(http.StatusUnauthorized).Send(t)
	})

	t.Run("weak_new_password", func(t *testing.T) {
		t.Parallel()

		server := builtinServer(t)
		token := loginAndGetToken(t, server, "admin", "adminpass")

		server.Client().Post("/api/v1/auth/change-password", api.ChangePasswordRequest{
			CurrentPassword: "adminpass",
			NewPassword:     "short",
		}).WithBearerToken(token).ExpectStatus(http.StatusBadRequest).Send(t)
	})

	t.Run("unauthenticated", func(t *testing.T) {
		t.Parallel()

		server := builtinServer(t)

		server.Client().Post("/api/v1/auth/change-password", api.ChangePasswordRequest{
			CurrentPassword: "adminpass",
			NewPassword:     "newpassword123",
		}).ExpectStatus(http.StatusUnauthorized).Send(t)
	})
}
