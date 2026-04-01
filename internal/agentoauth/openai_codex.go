// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agentoauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	openAICodexClientID     = "app_EMoamEEZ73f0CkXaXp7hrann"
	openAICodexAuthorizeURL = "https://auth.openai.com/oauth/authorize"
	openAICodexTokenURL     = "https://auth.openai.com/oauth/token"
	openAICodexRedirectURI  = "http://localhost:1455/auth/callback"
	openAICodexScope        = "openid profile email offline_access"
	openAICodexJWTClaimPath = "https://api.openai.com/auth"
	openAICodexHTTPTimeout  = 30 * time.Second
)

type openAICodexProvider struct{}

func (openAICodexProvider) ID() string {
	return ProviderOpenAICodex
}

func (openAICodexProvider) Name() string {
	return "ChatGPT Plus/Pro (Codex Subscription)"
}

func (openAICodexProvider) StartAuthFlow(_ context.Context) (*oauthFlow, error) {
	verifier, challenge, err := generatePKCE()
	if err != nil {
		return nil, err
	}
	state, err := randomHex(16)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(openAICodexAuthorizeURL)
	if err != nil {
		return nil, fmt.Errorf("build authorize URL: %w", err)
	}
	q := u.Query()
	q.Set("response_type", "code")
	q.Set("client_id", openAICodexClientID)
	q.Set("redirect_uri", openAICodexRedirectURI)
	q.Set("scope", openAICodexScope)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("state", state)
	q.Set("id_token_add_organizations", "true")
	q.Set("codex_cli_simplified_flow", "true")
	q.Set("originator", "dagu")
	u.RawQuery = q.Encode()

	return &oauthFlow{
		Verifier: verifier,
		State:    state,
		AuthURL:  u.String(),
	}, nil
}

func (openAICodexProvider) ExchangeCode(ctx context.Context, code, verifier string) (*Credential, error) {
	if strings.TrimSpace(code) == "" {
		return nil, fmt.Errorf("missing authorization code")
	}
	if strings.TrimSpace(verifier) == "" {
		return nil, fmt.Errorf("missing PKCE verifier")
	}

	form := url.Values{
		"grant_type":    {"authorization_code"},
		"client_id":     {openAICodexClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"redirect_uri":  {openAICodexRedirectURI},
	}
	return exchangeToken(ctx, form)
}

func (openAICodexProvider) Refresh(ctx context.Context, cred *Credential) (*Credential, error) {
	if cred == nil || strings.TrimSpace(cred.RefreshToken) == "" {
		return nil, fmt.Errorf("missing refresh token")
	}

	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {cred.RefreshToken},
		"client_id":     {openAICodexClientID},
	}
	return exchangeToken(ctx, form)
}

func parseAuthorizationInput(redirectURL, code string) (parsedCode, parsedState string, err error) {
	if value := strings.TrimSpace(redirectURL); value != "" {
		if u, parseErr := url.Parse(value); parseErr == nil {
			return strings.TrimSpace(u.Query().Get("code")), strings.TrimSpace(u.Query().Get("state")), nil
		}
		if strings.Contains(value, "code=") {
			params, parseErr := url.ParseQuery(value)
			if parseErr == nil {
				return strings.TrimSpace(params.Get("code")), strings.TrimSpace(params.Get("state")), nil
			}
		}
		return "", "", fmt.Errorf("invalid redirect URL")
	}

	if value := strings.TrimSpace(code); value != "" {
		if strings.Contains(value, "#") {
			parts := strings.SplitN(value, "#", 2)
			return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
		}
		return value, "", nil
	}
	return "", "", nil
}

func generatePKCE() (verifier, challenge string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate PKCE verifier: %w", err)
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	sum := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}

func randomHex(numBytes int) (string, error) {
	raw := make([]byte, numBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random state: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func exchangeToken(ctx context.Context, form url.Values) (*Credential, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openAICodexTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := (&http.Client{Timeout: openAICodexHTTPTimeout}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("exchange OAuth token: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 8*1024))
		return nil, fmt.Errorf("oauth token exchange failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if strings.TrimSpace(payload.AccessToken) == "" || strings.TrimSpace(payload.RefreshToken) == "" || payload.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token response missing required fields")
	}

	accountID, err := extractAccountID(payload.AccessToken)
	if err != nil {
		return nil, err
	}

	return &Credential{
		AccessToken:  payload.AccessToken,
		RefreshToken: payload.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(payload.ExpiresIn) * time.Second),
		AccountID:    accountID,
	}, nil
}

func extractAccountID(accessToken string) (string, error) {
	parts := strings.Split(accessToken, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid OAuth access token")
	}

	payload, err := decodeBase64URL(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode OAuth token: %w", err)
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("decode OAuth token claims: %w", err)
	}

	authClaims, _ := claims[openAICodexJWTClaimPath].(map[string]any)
	accountID, _ := authClaims["chatgpt_account_id"].(string)
	if strings.TrimSpace(accountID) == "" {
		return "", fmt.Errorf("oauth token missing chatgpt account ID")
	}
	return accountID, nil
}

func decodeBase64URL(value string) ([]byte, error) {
	if rem := len(value) % 4; rem != 0 {
		value += strings.Repeat("=", 4-rem)
	}
	decoded, err := base64.URLEncoding.DecodeString(value)
	if err == nil {
		return decoded, nil
	}
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(value, "="))
}
