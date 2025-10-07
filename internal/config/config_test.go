package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCleanBasePath(t *testing.T) {
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
			cleanServerBasePath(srv)
			assert.Equal(t, tt.expected, srv.BasePath)
		})
	}
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
