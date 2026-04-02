// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package clicontext

import (
	"context"
	"testing"

	"github.com/dagu-org/dagu/internal/cmn/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CRUDAndCurrent(t *testing.T) {
	t.Parallel()

	enc, err := crypto.NewEncryptor("test-key")
	require.NoError(t, err)

	store, err := NewStore(t.TempDir(), enc)
	require.NoError(t, err)

	current, err := store.Current(context.Background())
	require.NoError(t, err)
	assert.Equal(t, LocalContextName, current)

	err = store.Create(context.Background(), &Context{
		Name:           "prod",
		ServerURL:      "https://example.com",
		APIKey:         "dagu_test_123",
		Description:    "production",
		SkipTLSVerify:  true,
		TimeoutSeconds: 15,
	})
	require.NoError(t, err)

	item, err := store.Get(context.Background(), "prod")
	require.NoError(t, err)
	assert.Equal(t, "https://example.com", item.ServerURL)
	assert.Equal(t, "dagu_test_123", item.APIKey)
	assert.True(t, item.SkipTLSVerify)

	items, err := store.List(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, "prod", items[0].Name)

	require.NoError(t, store.Use(context.Background(), "prod"))
	current, err = store.Current(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "prod", current)

	require.NoError(t, store.Delete(context.Background(), "prod"))
	current, err = store.Current(context.Background())
	require.NoError(t, err)
	assert.Equal(t, LocalContextName, current)
}

func TestStore_ValidateContext(t *testing.T) {
	t.Parallel()

	enc, err := crypto.NewEncryptor("test-key")
	require.NoError(t, err)

	store, err := NewStore(t.TempDir(), enc)
	require.NoError(t, err)

	tests := []struct {
		name    string
		ctx     *Context
		wantErr string
	}{
		{
			name: "missing name",
			ctx: &Context{
				ServerURL: "https://example.com",
				APIKey:    "dagu_test",
			},
			wantErr: "context name is required",
		},
		{
			name: "reserved local",
			ctx: &Context{
				Name:      LocalContextName,
				ServerURL: "https://example.com",
				APIKey:    "dagu_test",
			},
			wantErr: "\"local\" is reserved",
		},
		{
			name: "invalid url",
			ctx: &Context{
				Name:      "prod",
				ServerURL: "://bad",
				APIKey:    "dagu_test",
			},
			wantErr: "invalid server URL",
		},
		{
			name: "invalid api key",
			ctx: &Context{
				Name:      "prod",
				ServerURL: "https://example.com",
				APIKey:    "token",
			},
			wantErr: "api key must use the dagu_ prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := store.ValidateContext(tt.ctx)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
