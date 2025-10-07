package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_cleanBasePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CleanPath",
			input:    "/dagu",
			expected: "/dagu",
		},
		{
			name:     "PathWithTrailingSlashes",
			input:    "////dagu//",
			expected: "/dagu",
		},
		{
			name:     "PathWithoutLeadingSlash",
			input:    "dagu",
			expected: "/dagu",
		},
		{
			name:     "RootPath",
			input:    "/",
			expected: "",
		},
		{
			name:     "EmptyPath",
			input:    "",
			expected: "",
		},
		{
			name:     "ComplexPath",
			input:    "//api//v1//",
			expected: "/api/v1",
		},
		{
			name:     "DotPathElements",
			input:    "/api/../v1/./test",
			expected: "/v1/test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := &Server{BasePath: tt.input}
			srv.cleanBasePath()
			assert.Equal(t, tt.expected, srv.BasePath)
		})
	}
}

func TestGlobal_setTimezone(t *testing.T) {
	t.Run("ValidTimezone", func(t *testing.T) {
		g := &Global{TZ: "America/New_York"}
		err := g.setTimezone()
		require.NoError(t, err)

		assert.Equal(t, "America/New_York", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, "America/New_York", g.Location.String())
		// New York is UTC-5 or UTC-4 depending on DST
		assert.NotEqual(t, 0, g.TzOffsetInSec)
	})

	t.Run("UTCTimezone", func(t *testing.T) {
		g := &Global{TZ: "UTC"}
		err := g.setTimezone()
		require.NoError(t, err)

		assert.Equal(t, "UTC", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 0, g.TzOffsetInSec)
	})

	t.Run("AsiaTokyoTimezone", func(t *testing.T) {
		g := &Global{TZ: "Asia/Tokyo"}
		err := g.setTimezone()
		require.NoError(t, err)

		assert.Equal(t, "Asia/Tokyo", g.TZ)
		assert.NotNil(t, g.Location)
		assert.Equal(t, 9*3600, g.TzOffsetInSec) // Tokyo is UTC+9
	})

	t.Run("InvalidTimezone", func(t *testing.T) {
		g := &Global{TZ: "Invalid/Timezone"}
		err := g.setTimezone()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load timezone")
	})

	t.Run("EmptyTimezoneUsesLocal", func(t *testing.T) {
		g := &Global{TZ: ""}
		err := g.setTimezone()
		require.NoError(t, err)

		// Should set TZ to UTC or UTC+X format
		assert.NotEmpty(t, g.TZ)
		assert.NotNil(t, g.Location)
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
			auth := AuthBasic{Username: tt.username, Password: tt.password}
			assert.Equal(t, tt.expected, auth.Enabled())
		})
	}
}

func TestAuthToken_Enabled(t *testing.T) {
	token := AuthToken{Value: "token"}
	assert.True(t, token.Enabled())

	emptyToken := AuthToken{Value: ""}
	assert.False(t, emptyToken.Enabled())
}

func TestAuthOIDC_Enabled(t *testing.T) {
	tests := []struct {
		name     string
		oidc     AuthOIDC
		expected bool
	}{
		{
			name: "AllRequiredFields",
			oidc: AuthOIDC{
				ClientId:     "id",
				ClientSecret: "secret",
				Issuer:       "https://issuer.com",
			},
			expected: true,
		},
		{
			name: "MissingClientId",
			oidc: AuthOIDC{
				ClientId:     "",
				ClientSecret: "secret",
				Issuer:       "https://issuer.com",
			},
			expected: false,
		},
		{
			name: "MissingClientSecret",
			oidc: AuthOIDC{
				ClientId:     "id",
				ClientSecret: "",
				Issuer:       "https://issuer.com",
			},
			expected: false,
		},
		{
			name: "MissingIssuer",
			oidc: AuthOIDC{
				ClientId:     "id",
				ClientSecret: "secret",
				Issuer:       "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.oidc.Enabled())
		})
	}
}

func TestTLSConfig_IsEnabled(t *testing.T) {
	tests := []struct {
		name     string
		tls      TLSConfig
		expected bool
	}{
		{
			name: "AllFieldsSet",
			tls: TLSConfig{
				CertFile: "/cert.pem",
				KeyFile:  "/key.pem",
				CAFile:   "/ca.pem",
			},
			expected: true,
		},
		{
			name: "MissingCertFile",
			tls: TLSConfig{
				CertFile: "",
				KeyFile:  "/key.pem",
				CAFile:   "/ca.pem",
			},
			expected: false,
		},
		{
			name: "MissingKeyFile",
			tls: TLSConfig{
				CertFile: "/cert.pem",
				KeyFile:  "",
				CAFile:   "/ca.pem",
			},
			expected: false,
		},
		{
			name: "MissingCAFile",
			tls: TLSConfig{
				CertFile: "/cert.pem",
				KeyFile:  "/key.pem",
				CAFile:   "",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.tls.IsEnabled())
		})
	}
}
