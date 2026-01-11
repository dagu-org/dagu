package auth

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	oidc "github.com/coreos/go-oidc"
	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/config"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/common/stringutil"
	authservice "github.com/dagu-org/dagu/internal/service/auth"
	"github.com/dagu-org/dagu/internal/service/oidcprovision"
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
	oidcProviderMaxRetries  = 3                      // max retries for OIDC provider init (network issues)
	oidcProviderRetryDelay  = 500 * time.Millisecond // delay between retries
	stateCookieExpiry       = 120                    // seconds for transient state/nonce/originalURL cookies
	defaultTokenExpirySecs  = 60                     // fallback when ID token expiry is invalid or already passed
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

// oidcProviderParams holds the common parameters for OIDC provider initialization.
type oidcProviderParams struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	ClientURL    string
	Scopes       []string
}

// oidcProviderResult holds the initialized OIDC components.
type oidcProviderResult struct {
	Provider     *oidc.Provider
	Verifier     *oidc.IDTokenVerifier
	OAuth2Config *oauth2.Config
}

// initOIDCProviderCore initializes the common OIDC provider components.
func initOIDCProviderCore(params oidcProviderParams) (*oidcProviderResult, error) {
	// Basic input validation
	if params.Issuer == "" {
		return nil, errors.New("failed to init OIDC provider: issuer is empty")
	}
	if params.ClientID == "" {
		return nil, errors.New("failed to init OIDC provider: client id is empty")
	}
	if params.ClientURL == "" {
		return nil, errors.New("failed to init OIDC provider: client url is empty")
	}

	ctx := oidc.ClientContext(context.Background(), &http.Client{
		Timeout: oidcProviderInitTimeout,
	})

	// Retry OIDC provider init in case of transient network errors
	var provider *oidc.Provider
	var err error
	for attempt := 1; attempt <= oidcProviderMaxRetries; attempt++ {
		provider, err = oidcProviderFactory(ctx, params.Issuer)
		if err == nil && provider != nil {
			break
		}
		if attempt < oidcProviderMaxRetries {
			slog.Warn("OIDC provider init failed, retrying",
				tag.Error(err),
				slog.Int("attempt", attempt),
				slog.Int("maxRetries", oidcProviderMaxRetries))
			time.Sleep(oidcProviderRetryDelay)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("failed to init OIDC provider: %w", err)
	}
	if provider == nil {
		return nil, errors.New("failed to init OIDC provider: provider is nil")
	}

	oidcConfig := &oidc.Config{
		ClientID: params.ClientID,
	}
	verifier := provider.Verifier(oidcConfig)
	endpoint := provider.Endpoint()

	oauth2Config := &oauth2.Config{
		ClientID:     params.ClientID,
		ClientSecret: params.ClientSecret,
		Endpoint:     endpoint,
		RedirectURL:  fmt.Sprintf("%s/oidc-callback", strings.TrimSuffix(params.ClientURL, "/")),
		Scopes:       params.Scopes,
	}

	return &oidcProviderResult{
		Provider:     provider,
		Verifier:     verifier,
		OAuth2Config: oauth2Config,
	}, nil
}

func InitVerifierAndConfig(i config.AuthOIDC) (_ *OIDCConfig, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("failed to init OIDC provider: %v", r)
		}
	}()

	result, err := initOIDCProviderCore(oidcProviderParams{
		Issuer:       i.Issuer,
		ClientID:     i.ClientId,
		ClientSecret: i.ClientSecret,
		ClientURL:    i.ClientUrl,
		Scopes:       i.Scopes,
	})
	if err != nil {
		return nil, err
	}

	return &OIDCConfig{
		Provider: result.Provider,
		Verifier: result.Verifier,
		Config:   result.OAuth2Config,
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
		clearOIDCStateCookies(w, r)
		http.Redirect(w, r, originalURL.Value, http.StatusFound)
	}
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

	var claims oidcprovision.OIDCClaims
	if err := token.Claims(&claims); err != nil {
		return nil, err
	}

	// Determine username with fallback: preferred_username -> email -> subject
	username := cmp.Or(claims.PreferredUsername, claims.Email, claims.Subject)

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
			// Add user and client IP to context
			ctx = auth.WithUser(ctx, user)
			ctx = auth.WithClientIP(ctx, GetClientIP(r))
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
	// Add user and client IP to context
	ctx = auth.WithUser(ctx, user)
	ctx = auth.WithClientIP(ctx, GetClientIP(r))
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

// clearOIDCStateCookies removes all OIDC-related state cookies.
func clearOIDCStateCookies(w http.ResponseWriter, r *http.Request) {
	clearCookie(w, r, cookieState)
	clearCookie(w, r, cookieNonce)
	clearCookie(w, r, cookieOriginalURL)
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	return strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

// BuiltinOIDCConfig holds configuration for OIDC under builtin auth mode.
type BuiltinOIDCConfig struct {
	Provider      *oidc.Provider
	Verifier      *oidc.IDTokenVerifier
	OAuth2Config  *oauth2.Config
	Provision     *oidcprovision.Service
	AuthService   *authservice.Service
	LoginBasePath string // Base path for login page redirect
}

// InitBuiltinOIDCConfig initializes OIDC for builtin auth mode.
func InitBuiltinOIDCConfig(cfg config.AuthOIDC, authSvc *authservice.Service, provisionSvc *oidcprovision.Service, basePath string) (*BuiltinOIDCConfig, error) {
	result, err := initOIDCProviderCore(oidcProviderParams{
		Issuer:       cfg.Issuer,
		ClientID:     cfg.ClientId,
		ClientSecret: cfg.ClientSecret,
		ClientURL:    cfg.ClientUrl,
		Scopes:       cfg.Scopes,
	})
	if err != nil {
		return nil, err
	}

	loginBasePath := basePath
	if loginBasePath == "" {
		loginBasePath = "/"
	}
	if !strings.HasPrefix(loginBasePath, "/") {
		loginBasePath = "/" + loginBasePath
	}

	return &BuiltinOIDCConfig{
		Provider:      result.Provider,
		Verifier:      result.Verifier,
		OAuth2Config:  result.OAuth2Config,
		Provision:     provisionSvc,
		AuthService:   authSvc,
		LoginBasePath: loginBasePath,
	}, nil
}

// BuiltinOIDCLoginHandler returns a handler that initiates the OIDC login flow.
func BuiltinOIDCLoginHandler(cfg *BuiltinOIDCConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		state := stringutil.RandomString(16)
		nonce := stringutil.RandomString(16)

		setCookie(w, r, cookieState, state, stateCookieExpiry)
		setCookie(w, r, cookieNonce, nonce, stateCookieExpiry)
		setCookie(w, r, cookieOriginalURL, cfg.LoginBasePath, stateCookieExpiry)

		http.Redirect(w, r, cfg.OAuth2Config.AuthCodeURL(state, oidc.Nonce(nonce)), http.StatusFound)
	}
}

// BuiltinOIDCCallbackHandler returns a handler that processes the OIDC callback.
// It uses the provisioning service to create/lookup users and generates JWT tokens.
func BuiltinOIDCCallbackHandler(cfg *BuiltinOIDCConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Verify state
		stateCookie, err := r.Cookie(cookieState)
		if err != nil {
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: state not found")
			return
		}
		if r.URL.Query().Get("state") != stateCookie.Value {
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: state mismatch")
			return
		}

		// Check for error from OIDC provider
		if errParam := r.URL.Query().Get("error"); errParam != "" {
			errDesc := r.URL.Query().Get("error_description")
			if errDesc == "" {
				errDesc = errParam
			}
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: "+errDesc)
			return
		}

		// Exchange code for token
		oauth2Token, err := cfg.OAuth2Config.Exchange(ctx, r.URL.Query().Get("code"))
		if err != nil {
			logger.Error(ctx, "OIDC token exchange failed", tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: could not exchange token")
			return
		}

		// Get raw ID token
		rawIDToken, ok := oauth2Token.Extra("id_token").(string)
		if !ok {
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: no ID token")
			return
		}

		// Verify ID token
		idToken, err := cfg.Verifier.Verify(ctx, rawIDToken)
		if err != nil {
			logger.Error(ctx, "OIDC token verification failed", tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: invalid token")
			return
		}

		// Verify nonce
		nonceCookie, err := r.Cookie(cookieNonce)
		if err != nil {
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: nonce not found")
			return
		}
		if idToken.Nonce != nonceCookie.Value {
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: nonce mismatch")
			return
		}

		// Extract typed claims
		var claims oidcprovision.OIDCClaims
		if err := idToken.Claims(&claims); err != nil {
			logger.Error(ctx, "Failed to extract OIDC claims", tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: could not read user info")
			return
		}

		// Extract raw claims for role mapping
		var rawClaims map[string]any
		if err := idToken.Claims(&rawClaims); err != nil {
			logger.Error(ctx, "Failed to extract raw OIDC claims", tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: could not read user info")
			return
		}
		claims.RawClaims = rawClaims

		// Get userinfo for email (may not be in ID token)
		userInfo, err := cfg.Provider.UserInfo(ctx, oauth2.StaticTokenSource(oauth2Token))
		if err != nil {
			logger.Warn(ctx, "Failed to get userinfo, using ID token claims", tag.Error(err))
			// Continue with ID token claims if userinfo fails
		} else {
			// Prefer userinfo email
			if userInfo.Email != "" {
				claims.Email = userInfo.Email
			}
			// Merge userinfo claims into raw claims for role mapping
			var userInfoClaims map[string]any
			if err := userInfo.Claims(&userInfoClaims); err == nil {
				for k, v := range userInfoClaims {
					if _, exists := claims.RawClaims[k]; !exists {
						claims.RawClaims[k] = v
					}
				}
			}
		}

		// Process login (create/lookup user)
		user, isNewUser, err := cfg.Provision.ProcessLogin(ctx, claims)
		if err != nil {
			logger.Warn(ctx, "OIDC provisioning failed",
				slog.String("email_domain", stringutil.ExtractEmailDomain(claims.Email)),
				slog.String("subject", claims.Subject),
				tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, err.Error())
			return
		}

		// Generate JWT token
		tokenResult, err := cfg.AuthService.GenerateToken(user)
		if err != nil {
			logger.Error(ctx, "Failed to generate JWT token", tag.Error(err))
			redirectWithError(w, r, cfg.LoginBasePath, "Authentication failed: could not create session")
			return
		}

		// Clear OIDC cookies
		clearOIDCStateCookies(w, r)

		// Redirect to login page with token in URL for frontend to store in localStorage
		// This is secure because:
		// 1. It's a one-time redirect (not a shareable link)
		// 2. Frontend stores the token and navigates away with replace:true (React Router)
		// 3. Token won't appear in browser history after navigation completes
		redirectURL := strings.TrimSuffix(cfg.LoginBasePath, "/") + "/login?token=" + url.QueryEscape(tokenResult.Token)
		if isNewUser {
			redirectURL += "&welcome=true"
		}
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

// redirectWithError redirects to the login page with an error message.
func redirectWithError(w http.ResponseWriter, r *http.Request, basePath, errMsg string) {
	clearOIDCStateCookies(w, r)

	redirectURL := strings.TrimSuffix(basePath, "/") + "/login?error=" + url.QueryEscape(errMsg)
	http.Redirect(w, r, redirectURL, http.StatusFound)
}
