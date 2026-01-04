package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	"golang.org/x/oauth2"
)

// OIDCConfig holds the initialized OIDC provider, verifier, and OAuth2 config
type OIDCConfig struct {
	Provider *oidc.Provider
	Verifier *oidc.IDTokenVerifier
	Config   *oauth2.Config
}

// Tunable constants for OIDC auth behaviour.
const (
	oidcProviderInitTimeout = 10 * time.Second
	stateCookieExpiry       = 120 // seconds for transient state/nonce/originalURL cookies
	defaultTokenExpirySecs  = 60  // fallback when ID token expiry is invalid or already passed
)

// Cookie names centralised to avoid copy-paste strings.
const (
	cookieOIDCToken   = "oidcToken"
	cookieState       = "state"
	cookieNonce       = "nonce"
	cookieOriginalURL = "originalURL"
)

var oidcProviderFactory = func(ctx context.Context, issuer string) (*oidc.Provider, error) {
	return oidc.NewProvider(ctx, issuer)
}

func InitVerifierAndConfig(i config.AuthOIDC) (_ *OIDCConfig, err error) {
	// Basic input validation to fail fast with clearer diagnostics.
	if i.Issuer == "" {
		return nil, errors.New("failed to init OIDC provider: issuer is empty")
	}
	if i.ClientId == "" {
		return nil, errors.New("failed to init OIDC provider: client id is empty")
	}
	if i.ClientUrl == "" {
		return nil, errors.New("failed to init OIDC provider: client url is empty")
	}
	// ClientSecret may be empty for public clients; don't enforce.

	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to init OIDC provider: %v", r)
		}
	}()

	ctx := oidc.ClientContext(context.Background(), &http.Client{
		Timeout: oidcProviderInitTimeout,
	})

	provider, err := oidcProviderFactory(ctx, i.Issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to init OIDC provider: %w", err)
	}
	if provider == nil {
		return nil, errors.New("failed to init OIDC provider: provider is nil")
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

// callbackHandler returns a handler for processing the OIDC redirect.
// The whitelist slice is converted once to a lookup map for efficiency.
func callbackHandler(provider *oidc.Provider, verifier *oidc.IDTokenVerifier,
	config *oauth2.Config, whitelist []string) func(w http.ResponseWriter, r *http.Request) {
	var allowedEmails map[string]struct{}
	if len(whitelist) > 0 {
		allowedEmails = make(map[string]struct{}, len(whitelist))
		for _, e := range whitelist {
			allowedEmails[e] = struct{}{}
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := context.Background()
		stateCookie, err := r.Cookie(cookieState)
		if err != nil {
			http.Error(w, "state not found", http.StatusBadRequest)
			return
		}
		originalURL, err := r.Cookie(cookieOriginalURL)
		if err != nil {
			http.Error(w, "original url not found", http.StatusBadRequest)
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
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
		if allowedEmails != nil {
			if _, ok := allowedEmails[userInfo.Email]; !ok {
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
		nonceCookie, err := r.Cookie(cookieNonce)
		if err != nil {
			http.Error(w, "nonce not found", http.StatusBadRequest)
			return
		}
		if idToken.Nonce != nonceCookie.Value {
			http.Error(w, "nonce did not match", http.StatusBadRequest)
			return
		}
		expireSeconds := int(time.Until(idToken.Expiry).Seconds())
		if expireSeconds <= 0 {
			expireSeconds = defaultTokenExpirySecs
		}
		setCookie(w, r, cookieOIDCToken, rawIDToken, expireSeconds)
		clearCookie(w, r, cookieState)
		clearCookie(w, r, cookieNonce)
		clearCookie(w, r, cookieOriginalURL)
		http.Redirect(w, r, originalURL.Value, http.StatusFound)
	}
}

// oidcClaims represents the claims extracted from an OIDC ID token.
type oidcClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	PreferredUsername string `json:"preferred_username"`
	Name              string `json:"name"`
}

// verifyIDToken verifies an ID token string using the provided verifier.
// Returns nil if valid, error otherwise.
func verifyIDToken(verifier *oidc.IDTokenVerifier, raw string) error {
	if verifier == nil {
		return errors.New("verifier is nil")
	}
	_, err := verifier.Verify(context.Background(), raw)
	return err
}

// verifyAndExtractUser verifies an ID token and extracts user information.
func verifyAndExtractUser(verifier *oidc.IDTokenVerifier, raw string) (*auth.User, error) {
	if verifier == nil {
		return nil, errors.New("verifier is nil")
	}
	token, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		return nil, err
	}

	var claims oidcClaims
	if err := token.Claims(&claims); err != nil {
		return nil, err
	}

	// Determine username: prefer preferred_username, then email, then subject
	username := claims.PreferredUsername
	if username == "" {
		username = claims.Email
	}
	if username == "" {
		username = claims.Subject
	}

	return &auth.User{
		ID:       claims.Subject,
		Username: username,
		Role:     auth.RoleAdmin, // OIDC users get admin by default
	}, nil
}

func checkOIDCAuth(next http.Handler, provider *oidc.Provider, verifier *oidc.IDTokenVerifier,
	config *oauth2.Config, whitelist []string, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Fast-path: already authenticated and not hitting the callback endpoint.
	if authorized, err := r.Cookie(cookieOIDCToken); err == nil && authorized.Value != "" && !strings.HasSuffix(config.RedirectURL, r.URL.Path) {
		if user, err := verifyAndExtractUser(verifier, authorized.Value); err == nil {
			// Add user to context
			ctx = auth.WithUser(ctx, user)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		logger.Warn(ctx, "OIDC cookie rejected: token verification failed during UI access",
			tag.Path(r.URL.Path),
			slog.String("method", r.Method))
		clearCookie(w, r, cookieOIDCToken)
	}
	// Callback handling.
	if strings.HasSuffix(config.RedirectURL, r.URL.Path) {
		callbackHandler(provider, verifier, config, whitelist)(w, r)
		return
	}
	// Initiate auth redirect.
	state, nonce := stringutil.RandomString(16), stringutil.RandomString(16)
	setCookie(w, r, cookieState, state, stateCookieExpiry)
	setCookie(w, r, cookieNonce, nonce, stateCookieExpiry)
	setCookie(w, r, cookieOriginalURL, r.URL.String(), stateCookieExpiry)
	http.Redirect(w, r, config.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
}

func checkOIDCToken(next http.Handler, verifier *oidc.IDTokenVerifier, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	authorized, err := r.Cookie(cookieOIDCToken)
	if err != nil {
		logger.Warn(ctx, "OIDC authentication failed: oidcToken cookie not found",
			tag.Path(r.URL.Path),
			slog.String("method", r.Method),
			tag.Error(err))
		http.Error(w, "Authentication required: OIDC token cookie not found", http.StatusUnauthorized)
		return
	}
	if authorized.Value == "" {
		logger.Warn(ctx, "OIDC authentication failed: oidcToken cookie is empty",
			tag.Path(r.URL.Path),
			slog.String("method", r.Method))
		http.Error(w, "Authentication required: OIDC token is empty", http.StatusUnauthorized)
		return
	}
	user, err := verifyAndExtractUser(verifier, authorized.Value)
	if err != nil {
		logger.Warn(ctx, "OIDC authentication failed: token verification failed",
			tag.Path(r.URL.Path),
			slog.String("method", r.Method),
			tag.Error(err))
		clearCookie(w, r, cookieOIDCToken)
		http.Error(w, "Authentication failed: invalid or expired OIDC token", http.StatusUnauthorized)
		return
	}
	// Add user to context
	ctx = auth.WithUser(ctx, user)
	next.ServeHTTP(w, r.WithContext(ctx))
}

func setCookie(w http.ResponseWriter, r *http.Request, name, value string, expire int) {
	c := &http.Cookie{
		Name:     name,
		Value:    value,
		MaxAge:   expire,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
		HttpOnly: true,
	}
	http.SetCookie(w, c)
}

func clearCookie(w http.ResponseWriter, r *http.Request, name string) {
	setCookie(w, r, name, "", -1)
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
