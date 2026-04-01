// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileagentoauth

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/agentoauth"
	"github.com/dagu-org/dagu/internal/cmn/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_SetGetListDelete(t *testing.T) {
	t.Parallel()

	enc, err := crypto.NewEncryptor("test-key")
	require.NoError(t, err)

	dir := t.TempDir()
	store, err := New(dir, enc)
	require.NoError(t, err)

	cred := &agentoauth.Credential{
		Provider:     agentoauth.ProviderOpenAICodex,
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    time.Now().UTC().Round(0),
		AccountID:    "acct-1",
		UpdatedAt:    time.Now().UTC().Round(0),
	}
	require.NoError(t, store.Set(context.Background(), cred))

	path := filepath.Join(dir, agentoauth.ProviderOpenAICodex+fileExtension)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "access-token")
	assert.NotContains(t, string(raw), "refresh-token")

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(filePermissions), info.Mode().Perm())
	}

	got, err := store.Get(context.Background(), agentoauth.ProviderOpenAICodex)
	require.NoError(t, err)
	assert.Equal(t, cred.Provider, got.Provider)
	assert.Equal(t, cred.AccessToken, got.AccessToken)
	assert.Equal(t, cred.RefreshToken, got.RefreshToken)
	assert.Equal(t, cred.AccountID, got.AccountID)
	assert.WithinDuration(t, cred.ExpiresAt, got.ExpiresAt, time.Second)

	entries, err := store.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, agentoauth.ProviderOpenAICodex, entries[0].Provider)

	require.NoError(t, store.Delete(context.Background(), agentoauth.ProviderOpenAICodex))
	_, err = store.Get(context.Background(), agentoauth.ProviderOpenAICodex)
	require.ErrorIs(t, err, agentoauth.ErrCredentialNotFound)
}

func TestStore_RejectsInvalidProviderName(t *testing.T) {
	t.Parallel()

	enc, err := crypto.NewEncryptor("test-key")
	require.NoError(t, err)

	store, err := New(t.TempDir(), enc)
	require.NoError(t, err)

	err = store.Set(context.Background(), &agentoauth.Credential{
		Provider:    "../bad",
		AccessToken: "token",
	})
	require.Error(t, err)
}
