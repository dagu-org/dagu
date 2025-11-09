package auth

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the initialized OIDC provider, verifier, and OAuth2 config
type OIDCConfig struct {
	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
	Config   *oauth2.Config
}

func InitVerifierAndConfig(i config.AuthOIDC) (*OIDCConfig, error) {
	providerCtx := context.Background()
	provider, err := oidc.NewProvider(providerCtx, i.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to init OIDC provider: %w", err)
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
	return &OIDCConfig{
		Provider: provider,
		Verifier: verifier,
		Config:   config,
	}, nil
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
		expireSeconds := int(time.Until(idToken.Expiry).Seconds())
		if expireSeconds <= 0 {
			// fall back to a short-lived cookie when token claims look off
			expireSeconds = 60
		}
		setCookie(w, r, "oidcToken", rawIDToken, expireSeconds)
		http.Redirect(w, r, originalURL.Value, http.StatusFound)
	}
}

func checkOIDCAuth(next http.Handler, provider *oidc.Provider, verifier *oidc.IDTokenVerifier,
	config *oauth2.Config, whitelist []string, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authorized, err := r.Cookie("oidcToken")
	if err == nil && authorized.Value != "" && !strings.HasSuffix(config.RedirectURL, r.URL.Path) {
		if verifier != nil {
			if _, err := verifier.Verify(ctx, authorized.Value); err == nil {
				next.ServeHTTP(w, r)
				return
			}
			logger.Warn(ctx, "OIDC cookie rejected: token verification failed during UI access",
				"path", r.URL.Path,
				"method", r.Method,
				"error", err)
			clearCookie(w, r, "oidcToken")
		} else {
			next.ServeHTTP(w, r)
			return
		}
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
	ctx := r.Context()
	authorized, err := r.Cookie("oidcToken")

	if err != nil {
		// Cookie not found or error reading it
		logger.Warn(ctx, "OIDC authentication failed: oidcToken cookie not found",
			"path", r.URL.Path,
			"method", r.Method,
			"error", err)
		http.Error(w, "Authentication required: OIDC token cookie not found", http.StatusUnauthorized)
		return
	}

	if authorized.Value == "" {
		logger.Warn(ctx, "OIDC authentication failed: oidcToken cookie is empty",
			"path", r.URL.Path,
			"method", r.Method)
		http.Error(w, "Authentication required: OIDC token is empty", http.StatusUnauthorized)
		return
	}

	// Verify the token
	if _, err := verifier.Verify(context.Background(), authorized.Value); err != nil {
		logger.Warn(ctx, "OIDC authentication failed: token verification failed",
			"path", r.URL.Path,
			"method", r.Method,
			"error", err)
		clearCookie(w, r, "oidcToken")
		http.Error(w, "Authentication failed: invalid or expired OIDC token", http.StatusUnauthorized)
		return
	}

	// Token is valid, proceed
	next.ServeHTTP(w, r)
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, expire int) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   expire,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	setCookie(w, r, name, "", -1)
}
