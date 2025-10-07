package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidPort_Negative", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: -1,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_TooLarge", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 99999,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("InvalidPort_MaxValue", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 65536,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port number")
	})

	t.Run("ValidPort_MinValue", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 0,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("ValidPort_MaxValue", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 65535,
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("IncompleteTLS_MissingCert", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					KeyFile: "/path/to/key.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("IncompleteTLS_MissingKey", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					CertFile: "/path/to/cert.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "TLS configuration incomplete")
	})

	t.Run("CompleteTLS", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
				TLS: &TLSConfig{
					CertFile: "/path/to/cert.pem",
					KeyFile:  "/path/to/key.pem",
				},
			},
			UI: UI{
				MaxDashboardPageLimit: 100,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("InvalidMaxDashboardPageLimit_Zero", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 0,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("InvalidMaxDashboardPageLimit_Negative", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: -1,
			},
		}
		err := cfg.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid max dashboard page limit")
	})

	t.Run("ValidMaxDashboardPageLimit_MinValue", func(t *testing.T) {
		cfg := &Config{
			Server: Server{
				Port: 8080,
			},
			UI: UI{
				MaxDashboardPageLimit: 1,
			},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})
}

func TestAuthBasic_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		expected bool
	}{
		{"BothSet", "admin", "secret", true},
		{"EmptyPassword", "admin", "", false},
		{"EmptyUsername", "", "secret", false},
		{"BothEmpty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := AuthBasic{
				Username: tt.username,
				Password: tt.password,
			}
			assert.Equal(t, tt.expected, auth.Enabled())
		})
	}
}

func TestAuthToken_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		expected bool
	}{
		{"TokenSet", "my-secret-token", true},
		{"EmptyToken", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := AuthToken{Value: tt.token}
			assert.Equal(t, tt.expected, auth.Enabled())
		})
	}
}

func TestAuthOIDC_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		oidc     AuthOIDC
		expected bool
	}{
		{
			name: "AllFieldsSet",
			oidc: AuthOIDC{
				ClientId:     "client-id",
				ClientSecret: "client-secret",
				Issuer:       "https://issuer.example.com",
			},
			expected: true,
		},
		{
			name: "MissingClientId",
			oidc: AuthOIDC{
				ClientId:     "",
				ClientSecret: "client-secret",
				Issuer:       "https://issuer.example.com",
			},
			expected: false,
		},
		{
			name: "MissingClientSecret",
			oidc: AuthOIDC{
				ClientId:     "client-id",
				ClientSecret: "",
				Issuer:       "https://issuer.example.com",
			},
			expected: false,
		},
		{
			name: "MissingIssuer",
			oidc: AuthOIDC{
				ClientId:     "client-id",
				ClientSecret: "client-secret",
				Issuer:       "",
			},
			expected: false,
		},
		{
			name:     "AllFieldsEmpty",
			oidc:     AuthOIDC{},
			expected: false,
		},
		{
			name: "WithOptionalFields",
			oidc: AuthOIDC{
				ClientId:     "client-id",
				ClientSecret: "client-secret",
				Issuer:       "https://issuer.example.com",
				ClientUrl:    "http://localhost:8081",
				Scopes:       []string{"openid", "profile"},
				Whitelist:    []string{"user@example.com"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.oidc.Enabled())
		})
	}
}
