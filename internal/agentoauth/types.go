// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agentoauth

import (
	"context"
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	ProviderOpenAICodex = "openai-codex"
)

var (
	ErrCredentialNotFound  = errors.New("credential not found")
	ErrFlowNotFound        = errors.New("oauth flow not found")
	ErrFlowExpired         = errors.New("oauth flow expired")
	ErrUnsupportedProvider = errors.New("unsupported oauth provider")
	ErrStateMismatch       = errors.New("oauth state mismatch")
)

// Credential stores subscription-backed OAuth credentials for a single provider.
type Credential struct {
	Provider     string    `json:"provider"`
	AccessToken  string    `json:"-"`
	RefreshToken string    `json:"-"`
	ExpiresAt    time.Time `json:"expiresAt"`
	AccountID    string    `json:"accountId,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// Store persists OAuth credentials for agent providers.
type Store interface {
	Get(ctx context.Context, provider string) (*Credential, error)
	Set(ctx context.Context, cred *Credential) error
	Delete(ctx context.Context, provider string) error
	List(ctx context.Context) ([]*Credential, error)
}

// ProviderStatus describes the connection state for one subscription provider.
type ProviderStatus struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Connected  bool       `json:"connected"`
	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	CanRefresh bool       `json:"canRefresh,omitempty"`
	AccountID  string     `json:"accountId,omitempty"`
}

// StartLoginResult contains the information needed to complete a manual OAuth flow.
type StartLoginResult struct {
	FlowID       string `json:"flowId"`
	AuthURL      string `json:"authUrl"`
	Instructions string `json:"instructions,omitempty"`
}

// CompleteLoginInput finalizes a login flow after the user pastes a redirect URL or code.
type CompleteLoginInput struct {
	FlowID      string `json:"flowId"`
	RedirectURL string `json:"redirectUrl,omitempty"`
	Code        string `json:"code,omitempty"`
}

type oauthProvider interface {
	ID() string
	Name() string
	StartAuthFlow(ctx context.Context) (*oauthFlow, error)
	ExchangeCode(ctx context.Context, code, verifier string) (*Credential, error)
	Refresh(ctx context.Context, cred *Credential) (*Credential, error)
}

type oauthFlow struct {
	Verifier string
	State    string
	AuthURL  string
}

type credentialProvider struct {
	manager  *Manager
	provider string
}

func (p credentialProvider) ResolveCredential(ctx context.Context) (llm.OAuthCredential, error) {
	cred, err := p.manager.EnsureValid(ctx, p.provider)
	if err != nil {
		return llm.OAuthCredential{}, err
	}
	return llm.OAuthCredential{
		AccessToken: cred.AccessToken,
		AccountID:   cred.AccountID,
	}, nil
}
