// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agentoauth

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	mu    sync.Mutex
	creds map[string]*Credential
}

func newMemoryStore() *memoryStore {
	return &memoryStore{creds: make(map[string]*Credential)}
}

func (s *memoryStore) Get(_ context.Context, provider string) (*Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cred, ok := s.creds[provider]
	if !ok {
		return nil, ErrCredentialNotFound
	}
	copy := *cred
	return &copy, nil
}

func (s *memoryStore) Set(_ context.Context, cred *Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	copy := *cred
	s.creds[cred.Provider] = &copy
	return nil
}

func (s *memoryStore) Delete(_ context.Context, provider string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.creds, provider)
	return nil
}

func (s *memoryStore) List(_ context.Context) ([]*Credential, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]*Credential, 0, len(s.creds))
	for _, cred := range s.creds {
		copy := *cred
		result = append(result, &copy)
	}
	return result, nil
}

type stubProvider struct {
	startAuthFlow func(context.Context) (*oauthFlow, error)
	exchangeCode  func(context.Context, string, string) (*Credential, error)
	refresh       func(context.Context, *Credential) (*Credential, error)
	refreshCalls  atomic.Int32
}

func (p *stubProvider) ID() string   { return ProviderOpenAICodex }
func (p *stubProvider) Name() string { return "Test Provider" }

func (p *stubProvider) StartAuthFlow(ctx context.Context) (*oauthFlow, error) {
	return p.startAuthFlow(ctx)
}

func (p *stubProvider) ExchangeCode(ctx context.Context, code, verifier string) (*Credential, error) {
	return p.exchangeCode(ctx, code, verifier)
}

func (p *stubProvider) Refresh(ctx context.Context, cred *Credential) (*Credential, error) {
	p.refreshCalls.Add(1)
	return p.refresh(ctx, cred)
}

func TestManager_CompleteLoginStoresCredential(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	provider := &stubProvider{
		startAuthFlow: func(context.Context) (*oauthFlow, error) {
			return &oauthFlow{
				Verifier: "verifier-1",
				State:    "state-1",
				AuthURL:  "https://example.com/auth",
			}, nil
		},
		exchangeCode: func(_ context.Context, code, verifier string) (*Credential, error) {
			require.Equal(t, "code-1", code)
			require.Equal(t, "verifier-1", verifier)
			return &Credential{
				AccessToken:  "access-1",
				RefreshToken: "refresh-1",
				ExpiresAt:    time.Now().Add(30 * time.Minute),
				AccountID:    "acct-1",
			}, nil
		},
		refresh: func(context.Context, *Credential) (*Credential, error) {
			t.Fatal("refresh should not be called")
			return nil, nil
		},
	}

	manager := NewManager(store)
	manager.providers = map[string]oauthProvider{ProviderOpenAICodex: provider}
	manager.refreshLocks = map[string]*sync.Mutex{ProviderOpenAICodex: {}}

	result, err := manager.StartLogin(context.Background(), ProviderOpenAICodex)
	require.NoError(t, err)

	cred, err := manager.CompleteLogin(context.Background(), ProviderOpenAICodex, CompleteLoginInput{
		FlowID:      result.FlowID,
		RedirectURL: "http://localhost:1455/auth/callback?code=code-1&state=state-1",
	})
	require.NoError(t, err)
	assert.Equal(t, ProviderOpenAICodex, cred.Provider)
	assert.Equal(t, "acct-1", cred.AccountID)
	assert.False(t, cred.UpdatedAt.IsZero())

	stored, err := store.Get(context.Background(), ProviderOpenAICodex)
	require.NoError(t, err)
	assert.Equal(t, "access-1", stored.AccessToken)
	assert.Equal(t, "refresh-1", stored.RefreshToken)
}

func TestManager_CompleteLoginRejectsStateMismatch(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	manager := NewManager(store)
	manager.providers = map[string]oauthProvider{
		ProviderOpenAICodex: &stubProvider{
			startAuthFlow: func(context.Context) (*oauthFlow, error) {
				return &oauthFlow{
					Verifier: "verifier-1",
					State:    "expected-state",
					AuthURL:  "https://example.com/auth",
				}, nil
			},
			exchangeCode: func(context.Context, string, string) (*Credential, error) {
				t.Fatal("exchange should not be called")
				return nil, nil
			},
			refresh: func(context.Context, *Credential) (*Credential, error) {
				t.Fatal("refresh should not be called")
				return nil, nil
			},
		},
	}

	result, err := manager.StartLogin(context.Background(), ProviderOpenAICodex)
	require.NoError(t, err)

	_, err = manager.CompleteLogin(context.Background(), ProviderOpenAICodex, CompleteLoginInput{
		FlowID:      result.FlowID,
		RedirectURL: "http://localhost:1455/auth/callback?code=code-1&state=wrong-state",
	})
	require.ErrorIs(t, err, ErrStateMismatch)
}

func TestManager_EnsureValidRefreshesOnceConcurrently(t *testing.T) {
	t.Parallel()

	store := newMemoryStore()
	require.NoError(t, store.Set(context.Background(), &Credential{
		Provider:     ProviderOpenAICodex,
		AccessToken:  "expired-access",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().Add(-1 * time.Minute),
		AccountID:    "acct-1",
	}))

	refreshStarted := make(chan struct{})
	refreshDone := make(chan struct{})
	provider := &stubProvider{
		startAuthFlow: func(context.Context) (*oauthFlow, error) {
			return nil, nil
		},
		exchangeCode: func(context.Context, string, string) (*Credential, error) {
			return nil, nil
		},
		refresh: func(_ context.Context, cred *Credential) (*Credential, error) {
			close(refreshStarted)
			<-refreshDone
			return &Credential{
				Provider:     cred.Provider,
				AccessToken:  "fresh-access",
				RefreshToken: cred.RefreshToken,
				ExpiresAt:    time.Now().Add(30 * time.Minute),
				AccountID:    cred.AccountID,
			}, nil
		},
	}

	manager := NewManager(store)
	manager.providers = map[string]oauthProvider{ProviderOpenAICodex: provider}
	manager.refreshLocks = map[string]*sync.Mutex{ProviderOpenAICodex: {}}
	manager.refreshSkew = 0

	const goroutines = 8
	var wg sync.WaitGroup
	results := make(chan *Credential, goroutines)
	errors := make(chan error, goroutines)

	for range goroutines {
		wg.Go(func() {
			cred, err := manager.EnsureValid(context.Background(), ProviderOpenAICodex)
			if err != nil {
				errors <- err
				return
			}
			results <- cred
		})
	}

	<-refreshStarted
	close(refreshDone)

	wg.Wait()
	close(results)
	close(errors)

	for err := range errors {
		require.NoError(t, err)
	}
	for cred := range results {
		assert.Equal(t, "fresh-access", cred.AccessToken)
	}
	assert.Equal(t, int32(1), provider.refreshCalls.Load())
}
