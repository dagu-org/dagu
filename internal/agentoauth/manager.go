// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agentoauth

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/stringutil"
	"github.com/dagucloud/dagu/internal/llm"
)

const (
	defaultFlowTTL     = 15 * time.Minute
	defaultRefreshSkew = 2 * time.Minute
)

type pendingFlow struct {
	provider string
	verifier string
	state    string
	created  time.Time
}

// Manager coordinates OAuth login flows, credential refresh, and runtime resolution.
type Manager struct {
	store       Store
	providers   map[string]oauthProvider
	flowTTL     time.Duration
	refreshSkew time.Duration

	mu           sync.Mutex
	flows        map[string]pendingFlow
	refreshLocks map[string]*sync.Mutex
}

// NewManager creates a new OAuth manager for agent providers.
func NewManager(store Store) *Manager {
	return &Manager{
		store:       store,
		flowTTL:     defaultFlowTTL,
		refreshSkew: defaultRefreshSkew,
		flows:       make(map[string]pendingFlow),
		refreshLocks: map[string]*sync.Mutex{
			ProviderOpenAICodex: {},
		},
		providers: map[string]oauthProvider{
			ProviderOpenAICodex: openAICodexProvider{},
		},
	}
}

// CredentialProvider returns a dynamic credential provider for use by LLM providers.
func (m *Manager) CredentialProvider(provider string) llm.OAuthCredentialProvider {
	if m == nil {
		return nil
	}
	return credentialProvider{
		manager:  m,
		provider: provider,
	}
}

// StartLogin initializes a new manual OAuth flow for the given provider.
func (m *Manager) StartLogin(ctx context.Context, provider string) (*StartLoginResult, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("oauth manager not configured")
	}
	prov, err := m.lookupProvider(provider)
	if err != nil {
		return nil, err
	}
	flow, err := prov.StartAuthFlow(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cleanupExpiredFlows(now)
	flowID := stringutil.RandomString(24)
	m.flows[flowID] = pendingFlow{
		provider: provider,
		verifier: flow.Verifier,
		state:    flow.State,
		created:  now,
	}

	return &StartLoginResult{
		FlowID:  flowID,
		AuthURL: flow.AuthURL,
		Instructions: "Open the URL in your browser. After authentication, copy the full redirect URL " +
			"or the authorization code and paste it back into Dagu.",
	}, nil
}

// CompleteLogin finalizes a previously started login flow and persists the credential.
func (m *Manager) CompleteLogin(ctx context.Context, provider string, input CompleteLoginInput) (*Credential, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("oauth manager not configured")
	}
	if strings.TrimSpace(input.FlowID) == "" {
		return nil, ErrFlowNotFound
	}
	code, state, err := parseAuthorizationInput(input.RedirectURL, input.Code)
	if err != nil {
		return nil, err
	}
	if code == "" {
		return nil, fmt.Errorf("missing authorization code")
	}

	m.mu.Lock()
	flow, ok := m.flows[input.FlowID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrFlowNotFound
	}
	if time.Since(flow.created) > m.flowTTL {
		delete(m.flows, input.FlowID)
		m.mu.Unlock()
		return nil, ErrFlowExpired
	}
	if flow.provider != provider {
		m.mu.Unlock()
		return nil, ErrFlowNotFound
	}
	if state != "" && state != flow.state {
		m.mu.Unlock()
		return nil, ErrStateMismatch
	}
	m.mu.Unlock()

	prov, err := m.lookupProvider(provider)
	if err != nil {
		return nil, err
	}
	cred, err := prov.ExchangeCode(ctx, code, flow.verifier)
	if err != nil {
		return nil, err
	}
	cred.Provider = provider
	cred.UpdatedAt = time.Now()
	if err := m.store.Set(ctx, cred); err != nil {
		return nil, err
	}

	m.mu.Lock()
	delete(m.flows, input.FlowID)
	m.mu.Unlock()

	return cred, nil
}

// Logout removes a stored OAuth credential.
func (m *Manager) Logout(ctx context.Context, provider string) error {
	if m == nil || m.store == nil {
		return fmt.Errorf("oauth manager not configured")
	}
	if _, err := m.lookupProvider(provider); err != nil {
		return err
	}
	return m.store.Delete(ctx, provider)
}

// Status returns the known connection status for each supported provider.
func (m *Manager) Status(ctx context.Context) ([]ProviderStatus, error) {
	statuses := make([]ProviderStatus, 0, len(m.providers))
	providers := make([]string, 0, len(m.providers))
	for provider := range m.providers {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	for _, provider := range providers {
		status, err := m.ProviderStatus(ctx, provider, nil)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

// ProviderStatus returns the connection status for a single provider.
func (m *Manager) ProviderStatus(ctx context.Context, provider string, cred *Credential) (ProviderStatus, error) {
	prov, err := m.lookupProvider(provider)
	if err != nil {
		return ProviderStatus{}, err
	}

	status := ProviderStatus{
		ID:   prov.ID(),
		Name: prov.Name(),
	}
	if cred == nil && m.store != nil {
		cred, err = m.store.Get(ctx, provider)
		switch {
		case err == nil:
		case errors.Is(err, ErrCredentialNotFound):
			cred = nil
		default:
			return ProviderStatus{}, err
		}
	}
	if cred != nil {
		status.Connected = true
		status.AccountID = cred.AccountID
		status.CanRefresh = strings.TrimSpace(cred.RefreshToken) != ""
		if !cred.ExpiresAt.IsZero() {
			expiresAt := cred.ExpiresAt
			status.ExpiresAt = &expiresAt
		}
	}
	return status, nil
}

// EnsureValid returns a valid credential for the provider, refreshing it if needed.
func (m *Manager) EnsureValid(ctx context.Context, provider string) (*Credential, error) {
	if m == nil || m.store == nil {
		return nil, fmt.Errorf("oauth manager not configured")
	}
	prov, err := m.lookupProvider(provider)
	if err != nil {
		return nil, err
	}

	cred, err := m.store.Get(ctx, provider)
	if err != nil {
		return nil, err
	}
	if cred == nil {
		return nil, ErrCredentialNotFound
	}

	if !m.needsRefresh(cred) {
		return cred, nil
	}
	if strings.TrimSpace(cred.RefreshToken) == "" {
		return nil, fmt.Errorf("oauth credential expired for provider %s", provider)
	}

	lock := m.refreshLock(provider)
	lock.Lock()
	defer lock.Unlock()

	latest, err := m.store.Get(ctx, provider)
	if err != nil {
		return nil, err
	}
	if latest != nil && !m.needsRefresh(latest) {
		return latest, nil
	}
	if latest == nil {
		return nil, ErrCredentialNotFound
	}

	refreshed, err := prov.Refresh(ctx, latest)
	if err != nil {
		return nil, err
	}
	refreshed.Provider = provider
	refreshed.UpdatedAt = time.Now()
	if err := m.store.Set(ctx, refreshed); err != nil {
		return nil, err
	}
	return refreshed, nil
}

func (m *Manager) lookupProvider(provider string) (oauthProvider, error) {
	if m == nil {
		return nil, fmt.Errorf("oauth manager not configured")
	}
	prov, ok := m.providers[provider]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedProvider, provider)
	}
	return prov, nil
}

func (m *Manager) needsRefresh(cred *Credential) bool {
	if cred == nil {
		return true
	}
	if cred.ExpiresAt.IsZero() {
		return false
	}
	return time.Until(cred.ExpiresAt) <= m.refreshSkew
}

func (m *Manager) refreshLock(provider string) *sync.Mutex {
	m.mu.Lock()
	defer m.mu.Unlock()
	lock, ok := m.refreshLocks[provider]
	if !ok {
		lock = &sync.Mutex{}
		m.refreshLocks[provider] = lock
	}
	return lock
}

func (m *Manager) cleanupExpiredFlows(now time.Time) {
	for id, flow := range m.flows {
		if now.Sub(flow.created) > m.flowTTL {
			delete(m.flows, id)
		}
	}
}
