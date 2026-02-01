package auth

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/stretchr/testify/require"
)

func TestInitVerifierAndConfigRecoversFromPanic(t *testing.T) {
	t.Cleanup(func() {
		oidcProviderFactory = oidc.NewProvider
	})
	oidcProviderFactory = func(context.Context, string) (*oidc.Provider, error) {
		panic("boom")
	}

	_, err := InitVerifierAndConfig(context.Background(), config.AuthOIDC{
		ClientID:     "client",
		ClientSecret: "secret",
		ClientURL:    "http://localhost",
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

	_, err := InitVerifierAndConfig(context.Background(), config.AuthOIDC{
		ClientID:     "client",
		ClientSecret: "secret",
		ClientURL:    "http://localhost",
		Issuer:       "http://issuer",
		Scopes:       []string{oidc.ScopeOpenID},
	})

	require.EqualError(t, err, "failed to init OIDC provider: provider is nil")
}

func TestInitVerifierAndConfigKeepsProviderContextAlive(t *testing.T) {
	server := newTestOIDCServer(t)
	t.Cleanup(server.Close)

	originalFactory := oidcProviderFactory
	t.Cleanup(func() {
		oidcProviderFactory = originalFactory
	})

	var capturedCtx context.Context
	oidcProviderFactory = func(ctx context.Context, issuer string) (*oidc.Provider, error) {
		capturedCtx = ctx
		return originalFactory(ctx, issuer)
	}

	cfg, err := InitVerifierAndConfig(context.Background(), config.AuthOIDC{
		ClientID:     "client",
		ClientSecret: "secret",
		ClientURL:    "http://localhost",
		Issuer:       server.URL,
		Scopes:       []string{oidc.ScopeOpenID},
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)
	require.NotNil(t, capturedCtx)

	select {
	case <-capturedCtx.Done():
		t.Fatalf("provider context cancelled: %v", capturedCtx.Err())
	default:
	}
}

type testOIDCHandler struct {
	issuer string
	ready  chan struct{}
}

func (h *testOIDCHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	<-h.ready

	switch r.URL.Path {
	case "/.well-known/openid-configuration":
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{
			"issuer": %q,
			"authorization_endpoint": %q,
			"token_endpoint": %q,
			"jwks_uri": %q,
			"userinfo_endpoint": %q,
			"id_token_signing_alg_values_supported": ["RS256"]
		}`, h.issuer, h.issuer+"/authorize", h.issuer+"/token", h.issuer+"/.well-known/jwks.json", h.issuer+"/userinfo")
	case "/.well-known/jwks.json":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"keys":[]}`))
	default:
		http.NotFound(w, r)
	}
}

func newTestOIDCServer(t *testing.T) *httptest.Server {
	t.Helper()

	handler := &testOIDCHandler{
		ready: make(chan struct{}),
	}
	server := httptest.NewServer(handler)
	handler.issuer = server.URL
	close(handler.ready)
	return server
}
