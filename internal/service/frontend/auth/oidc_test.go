package auth

import (
	"context"
	"testing"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/stretchr/testify/require"
)

func TestInitVerifierAndConfigRecoversFromPanic(t *testing.T) {
	t.Cleanup(func() {
		oidcProviderFactory = oidc.NewProvider
	})
	oidcProviderFactory = func(context.Context, string) (*oidc.Provider, error) {
		panic("boom")
	}

	_, err := InitVerifierAndConfig(config.AuthOIDC{
		ClientId:     "client",
		ClientSecret: "secret",
		ClientUrl:    "http://localhost",
		Issuer:       "http://issuer",
		Scopes:       []string{oidc.ScopeOpenID},
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestInitVerifierAndConfigHandlesNilProvider(t *testing.T) {
	t.Cleanup(func() {
		oidcProviderFactory = oidc.NewProvider
	})
	oidcProviderFactory = func(context.Context, string) (*oidc.Provider, error) {
		return nil, nil
	}

	_, err := InitVerifierAndConfig(config.AuthOIDC{
		ClientId:     "client",
		ClientSecret: "secret",
		ClientUrl:    "http://localhost",
		Issuer:       "http://issuer",
		Scopes:       []string{oidc.ScopeOpenID},
	})

	require.EqualError(t, err, "failed to init OIDC provider: provider is nil")
}
