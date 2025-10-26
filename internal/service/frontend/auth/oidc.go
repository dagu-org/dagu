package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"golang.org/x/oauth2"
)

func InitVerifierAndConfig(i config.AuthOIDC) (*oidc.Provider, *oidc.IDTokenVerifier, *oauth2.Config, error) {
	providerCtx := context.Background()
	provider, err := oidc.NewProvider(providerCtx, i.Issuer)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init OIDC provider: %w", err)
	}
	oidcConfig := &oidc.Config{
		ClientID: i.ClientId,
	}
	verifier := provider.Verifier(oidcConfig)
	endpoint := provider.Endpoint()
	config := &oauth2.Config{
		ClientID:     i.ClientId,
		ClientSecret: i.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  fmt.Sprintf("%s/oidc-callback", strings.TrimSuffix(i.ClientUrl, "/")),
		Scopes:       i.Scopes,
	}
	return provider, verifier, config, nil
}

func callbackHandler(provider *oidc.Provider, verifier *oidc.IDTokenVerifier,
	config *oauth2.Config, whitelist []string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		state, err := r.Cookie("state")
		if err != nil {
			http.Error(w, "state not found", http.StatusBadRequest)
			return
		}
		originalURL, err := r.Cookie("originalURL")
		if err != nil {
			http.Error(w, "original url not found", http.StatusBadRequest)
			return
		}
		// verify and exchange token
		if r.URL.Query().Get("state") != state.Value {
			http.Error(w, "state did not match", http.StatusBadRequest)
			return
		}
		oauth2Token, err := config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			http.Error(w, "Failed to exchange token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		userInfo, err := provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
		if err != nil {
			http.Error(w, "Failed to get userinfo: "+err.Error(), http.StatusInternalServerError)
			return
		}
		// default allow all oidc user, if whitelist is not empty only allow listed user.
		if len(whitelist) > 0 {
			allow := false
			for _, item := range whitelist {
				if item == userInfo.Email {
					allow = true
					break
				}
			}
			if !allow {
				http.Error(w, "No permissions", http.StatusForbidden)
				return
			}
		}
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			http.Error(w, "No id_token field in oauth2 token.", http.StatusInternalServerError)
			return
		}
		idToken, err := verifier.Verify(ctx, rawIDToken)
		if err != nil {
			http.Error(w, "Failed to verify ID Token: "+err.Error(), http.StatusInternalServerError)
			return
		}
		nonce, err := r.Cookie("nonce")
		if err != nil {
			http.Error(w, "nonce not found", http.StatusBadRequest)
			return
		}
		if idToken.Nonce != nonce.Value {
			http.Error(w, "nonce did not match", http.StatusBadRequest)
			return
		}
		// set token
		setCookie(w, r, "oidcToken", rawIDToken, 86400-120) //expire at 1 day
		http.Redirect(w, r, originalURL.Value, http.StatusFound)
	}
}

func checkOIDCAuth(next http.Handler, provider *oidc.Provider, verifier *oidc.IDTokenVerifier,
	config *oauth2.Config, whitelist []string, w http.ResponseWriter, r *http.Request) {
	authorized, err := r.Cookie("oidcToken")
	if err == nil && authorized.Value != "" && !strings.HasSuffix(config.RedirectURL, r.URL.Path) {
		next.ServeHTTP(w, r)
		return
	}
	// auth callback
	if strings.HasSuffix(config.RedirectURL, r.URL.Path) {
		callbackHandler(provider, verifier, config, whitelist)(w, r)
		return
	}
	// redirect to oidc
	state, nonce := stringutil.RandomString(16), stringutil.RandomString(16)
	setCookie(w, r, "state", state, 120)
	setCookie(w, r, "nonce", nonce, 120)
	setCookie(w, r, "originalURL", r.URL.String(), 120)
	http.Redirect(w, r, config.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func checkOIDCToken(next http.Handler, verifier *oidc.IDTokenVerifier, w http.ResponseWriter, r *http.Request) {
	authorized, err := r.Cookie("oidcToken")
	if err == nil && authorized.Value != "" {
		if _, err := verifier.Verify(context.Background(), authorized.Value); err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
		return
	}
	w.WriteHeader(http.StatusUnauthorized)
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, expire int) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   expire,
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}
