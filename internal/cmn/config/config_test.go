package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validBaseConfig returns a minimal valid Config for use in tests.
// Callers can mutate the returned config to set up specific test scenarios.
func validBaseConfig() *Config {
	return &Config{
		DefaultExecutionMode: ExecutionModeLocal,
		Server: Server{
			Port: 8080,
			Auth: Auth{Mode: AuthModeNone},
		},
		UI: UI{MaxDashboardPageLimit: 100},
	}
}

func TestConfig_Validate(t *testing.T) {
	t.Parallel()
	t.Run("ValidConfig", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidPort_Negative", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Port = -1
		cfg.Server.Auth = Auth{}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_TooLarge", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Port = 99999
		cfg.Server.Auth = Auth{}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_MaxValue", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Port = 65536
		cfg.Server.Auth = Auth{}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("ValidPort_MinValue", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Port = 0
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("ValidPort_MaxValue", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Port = 65535
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("IncompleteTLS_MissingCert", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.TLS = &TLSConfig{
			KeyFile: "/path/to/key.pem",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("IncompleteTLS_MissingKey", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.TLS = &TLSConfig{
			CertFile: "/path/to/cert.pem",
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("CompleteTLS", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.TLS = &TLSConfig{
			CertFile: "/path/to/cert.pem",
			KeyFile:  "/path/to/key.pem",
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidMaxDashboardPageLimit_Zero", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.UI.MaxDashboardPageLimit = 0
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("InvalidMaxDashboardPageLimit_Negative", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.UI.MaxDashboardPageLimit = -1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("ValidMaxDashboardPageLimit_MinValue", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("BuiltinAuth_MissingUsersDir", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
		}
		cfg.Paths.UsersDir = ""
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "usersDir")
	})

	t.Run("BuiltinAuth_MissingTokenSecret", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "", TTL: 1},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "token secret")
	})

	t.Run("BuiltinAuth_MissingAdminUsername", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: ""},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "admin username")
	})

	t.Run("BuiltinAuth_ValidConfig", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("BuiltinAuth_SkippedForOtherModes", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidAuthMode", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth.Mode = "invalid"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid auth mode")
	})

	t.Run("BuiltinAuth_InvalidTokenTTL", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 0},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "positive token TTL")
	})

	// Tests for incomplete OIDC config - with auto-detection, missing required fields
	// means OIDC is simply not enabled (no error). This is intentional.
	t.Run("BuiltinAuth_OIDC_IncompleteConfig_NoError", func(t *testing.T) {
		t.Parallel()
		// Missing clientId - OIDC is not enabled, no error
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
			OIDC: AuthOIDC{
				ClientID:     "", // Missing
				ClientSecret: "secret",
				ClientURL:    "https://example.com",
				Issuer:       "https://issuer.com",
				RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err, "incomplete OIDC config should not error - OIDC is simply not enabled")
		assert.False(t, cfg.Server.Auth.OIDC.IsConfigured(), "OIDC should not be configured")
	})

	t.Run("BuiltinAuth_OIDC_InvalidDefaultRole", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
			OIDC: AuthOIDC{
				ClientID:     "client-id",
				ClientSecret: "secret",
				ClientURL:    "https://example.com",
				Issuer:       "https://issuer.com",
				RoleMapping:  OIDCRoleMapping{DefaultRole: "invalid-role"},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "defaultRole")
	})

	t.Run("BuiltinAuth_OIDC_ValidConfig", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
			OIDC: AuthOIDC{
				ClientID:     "client-id",
				ClientSecret: "secret",
				ClientURL:    "https://example.com",
				Issuer:       "https://issuer.com",
				Scopes:       []string{"openid", "profile", "email"},
				RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("BuiltinAuth_OIDC_MissingEmailScope_AddsWarning", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
			OIDC: AuthOIDC{
				ClientID:     "client-id",
				ClientSecret: "secret",
				ClientURL:    "https://example.com",
				Issuer:       "https://issuer.com",
				Scopes:       []string{"openid", "profile"}, // No email scope
				RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Len(t, cfg.Warnings, 1)
		assert.Contains(t, cfg.Warnings[0], "email")
	})

	t.Run("BuiltinAuth_OIDC_MissingEmailScope_WithWhitelist_ReturnsError", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode: AuthModeBuiltin,
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
			OIDC: AuthOIDC{
				ClientID:     "client-id",
				ClientSecret: "secret",
				ClientURL:    "https://example.com",
				Issuer:       "https://issuer.com",
				Scopes:       []string{"openid", "profile"}, // No email scope
				Whitelist:    []string{"user@example.com"},  // But whitelist is set
				RoleMapping:  OIDCRoleMapping{DefaultRole: "viewer"},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.UI.MaxDashboardPageLimit = 1
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "email")
	})

	t.Run("BuiltinAuth_OIDC_AllValidRoles", func(t *testing.T) {
		t.Parallel()
		validRoles := []string{"admin", "manager", "operator", "viewer"}
		for _, role := range validRoles {
			cfg := validBaseConfig()
			cfg.Server.Auth = Auth{
				Mode: AuthModeBuiltin,
				Builtin: AuthBuiltin{
					Admin: AdminConfig{Username: "admin"},
					Token: TokenConfig{Secret: "secret", TTL: 1},
				},
				OIDC: AuthOIDC{
					ClientID:     "client-id",
					ClientSecret: "secret",
					ClientURL:    "https://example.com",
					Issuer:       "https://issuer.com",
					Scopes:       []string{"openid", "email"},
					RoleMapping:  OIDCRoleMapping{DefaultRole: role},
				},
			}
			cfg.Paths.UsersDir = "/tmp/users"
			cfg.UI.MaxDashboardPageLimit = 1
			err := cfg.Validate()
			require.NoError(t, err, "role %s should be valid", role)
		}
	})

	// Tunnel validation tests
	t.Run("TunnelPublicRequiresAuth", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Tunnel = TunnelConfig{
			Enabled: true,
			Tailscale: TailscaleTunnelConfig{
				Funnel: true, // Public access
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "requires authentication")
	})

	t.Run("TunnelPublicWithAuthOK", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Server.Auth = Auth{
			Mode:  AuthModeBuiltin,
			Basic: AuthBasic{Username: "user", Password: "pass"},
			Builtin: AuthBuiltin{
				Admin: AdminConfig{Username: "admin"},
				Token: TokenConfig{Secret: "secret", TTL: 1},
			},
		}
		cfg.Paths.UsersDir = "/tmp/users"
		cfg.Tunnel = TunnelConfig{
			Enabled: true,
			Tailscale: TailscaleTunnelConfig{
				Funnel: true, // Public access
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("TunnelPrivateNoAuthOK", func(t *testing.T) {
		t.Parallel()
		cfg := validBaseConfig()
		cfg.Tunnel = TunnelConfig{
			Enabled: true,
			Tailscale: TailscaleTunnelConfig{
				Funnel: false, // Private (tailnet only)
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("TunnelDisabledNoValidation", func(t *testing.T) {
		t.Parallel()
		// When tunnel is disabled, validation should pass regardless of settings
		cfg := validBaseConfig()
		cfg.Tunnel = TunnelConfig{
			Enabled: false,
			Tailscale: TailscaleTunnelConfig{
				Funnel: true, // Would require auth if enabled
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})
}
