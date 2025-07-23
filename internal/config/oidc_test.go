package config_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_OIDCEnvironmentVariables(t *testing.T) {
	// Reset viper to ensure a clean state
	viper.Reset()

	// Save original environment variables
	envVars := []string{
		"DAGU_AUTH_OIDC_CLIENT_ID",
		"DAGU_AUTH_OIDC_CLIENT_SECRET",
		"DAGU_AUTH_OIDC_CLIENT_URL",
		"DAGU_AUTH_OIDC_ISSUER",
		"DAGU_AUTH_OIDC_SCOPES",
		"DAGU_AUTH_OIDC_WHITELIST",
	}

	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
	}

	// Restore original environment after test
	defer func() {
		for envVar, value := range originalValues {
			if value == "" {
				os.Unsetenv(envVar)
			} else {
				os.Setenv(envVar, value)
			}
		}
	}()

	// Set OIDC environment variables
	os.Setenv("DAGU_AUTH_OIDC_CLIENT_ID", "test-client-id")
	os.Setenv("DAGU_AUTH_OIDC_CLIENT_SECRET", "test-client-secret")
	os.Setenv("DAGU_AUTH_OIDC_CLIENT_URL", "http://localhost:8080")
	os.Setenv("DAGU_AUTH_OIDC_ISSUER", "https://accounts.google.com")
	os.Setenv("DAGU_AUTH_OIDC_SCOPES", "openid,profile,email")
	os.Setenv("DAGU_AUTH_OIDC_WHITELIST", "user1@example.com,user2@example.com")

	// Load configuration
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify OIDC configuration
	assert.True(t, cfg.Server.Auth.OIDC.Enabled(), "OIDC should be enabled when required fields are set")
	assert.Equal(t, "test-client-id", cfg.Server.Auth.OIDC.ClientId)
	assert.Equal(t, "test-client-secret", cfg.Server.Auth.OIDC.ClientSecret)
	assert.Equal(t, "http://localhost:8080", cfg.Server.Auth.OIDC.ClientUrl)
	assert.Equal(t, "https://accounts.google.com", cfg.Server.Auth.OIDC.Issuer)
	assert.Equal(t, []string{"openid", "profile", "email"}, cfg.Server.Auth.OIDC.Scopes)
	assert.Equal(t, []string{"user1@example.com", "user2@example.com"}, cfg.Server.Auth.OIDC.Whitelist)
}

func TestLoadConfig_OIDCPartialEnvironmentVariables(t *testing.T) {
	// Reset viper to ensure a clean state
	viper.Reset()

	// Save original environment variables
	envVars := []string{
		"DAGU_AUTH_OIDC_CLIENT_ID",
		"DAGU_AUTH_OIDC_CLIENT_SECRET",
		"DAGU_AUTH_OIDC_ISSUER",
	}

	originalValues := make(map[string]string)
	for _, envVar := range envVars {
		originalValues[envVar] = os.Getenv(envVar)
		os.Unsetenv(envVar)
	}

	// Restore original environment after test
	defer func() {
		for envVar, value := range originalValues {
			if value == "" {
				os.Unsetenv(envVar)
			} else {
				os.Setenv(envVar, value)
			}
		}
	}()

	// Set only required OIDC fields
	os.Setenv("DAGU_AUTH_OIDC_CLIENT_ID", "minimal-client-id")
	os.Setenv("DAGU_AUTH_OIDC_CLIENT_SECRET", "minimal-secret")
	os.Setenv("DAGU_AUTH_OIDC_ISSUER", "https://auth.example.com")

	// Load configuration
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify OIDC configuration
	assert.True(t, cfg.Server.Auth.OIDC.Enabled(), "OIDC should be enabled with minimal required fields")
	assert.Equal(t, "minimal-client-id", cfg.Server.Auth.OIDC.ClientId)
	assert.Equal(t, "minimal-secret", cfg.Server.Auth.OIDC.ClientSecret)
	assert.Equal(t, "https://auth.example.com", cfg.Server.Auth.OIDC.Issuer)
	assert.Empty(t, cfg.Server.Auth.OIDC.ClientUrl, "ClientUrl should be empty when not set")
	assert.Empty(t, cfg.Server.Auth.OIDC.Scopes, "Scopes should be empty when not set")
	assert.Empty(t, cfg.Server.Auth.OIDC.Whitelist, "Whitelist should be empty when not set")
}

func TestLoadConfig_OIDCDisabled(t *testing.T) {
	// Reset viper to ensure a clean state
	viper.Reset()

	// Ensure no OIDC environment variables are set
	os.Unsetenv("DAGU_AUTH_OIDC_CLIENT_ID")
	os.Unsetenv("DAGU_AUTH_OIDC_CLIENT_SECRET")
	os.Unsetenv("DAGU_AUTH_OIDC_CLIENT_URL")
	os.Unsetenv("DAGU_AUTH_OIDC_ISSUER")
	os.Unsetenv("DAGU_AUTH_OIDC_SCOPES")
	os.Unsetenv("DAGU_AUTH_OIDC_WHITELIST")

	// Load configuration
	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	// Verify OIDC is disabled
	assert.False(t, cfg.Server.Auth.OIDC.Enabled(), "OIDC should be disabled when required fields are not set")
	assert.Empty(t, cfg.Server.Auth.OIDC.ClientId)
	assert.Empty(t, cfg.Server.Auth.OIDC.ClientSecret)
	assert.Empty(t, cfg.Server.Auth.OIDC.Issuer)
}