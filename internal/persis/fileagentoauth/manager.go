// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileagentoauth

import (
	"fmt"
	"path/filepath"

	"github.com/dagucloud/dagu/internal/agentoauth"
	"github.com/dagucloud/dagu/internal/cmn/crypto"
)

// NewManager creates an OAuth manager backed by the standard encrypted file store.
func NewManager(dataDir string) (*agentoauth.Manager, error) {
	encKey, err := crypto.ResolveKey(dataDir)
	if err != nil {
		return nil, fmt.Errorf("resolve encryption key for agent OAuth store: %w", err)
	}
	enc, err := crypto.NewEncryptor(encKey)
	if err != nil {
		return nil, fmt.Errorf("create encryptor for agent OAuth store: %w", err)
	}
	store, err := New(filepath.Join(dataDir, "agent", "oauth"), enc)
	if err != nil {
		return nil, fmt.Errorf("create agent OAuth store: %w", err)
	}
	return agentoauth.NewManager(store), nil
}
