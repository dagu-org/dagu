// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileapikey

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
)

func TestStore_CRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test Create
	apiKey := auth.NewAPIKey("test-key", "Test API key", auth.RoleManager, "hashedsecret", "dagu_xxx", "admin-user-id")
	if err := store.Create(ctx, apiKey); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test GetByID
	retrieved, err := store.GetByID(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if retrieved.Name != apiKey.Name {
		t.Errorf("GetByID() name = %v, want %v", retrieved.Name, apiKey.Name)
	}
	if retrieved.Role != apiKey.Role {
		t.Errorf("GetByID() role = %v, want %v", retrieved.Role, apiKey.Role)
	}
	if retrieved.Description != apiKey.Description {
		t.Errorf("GetByID() description = %v, want %v", retrieved.Description, apiKey.Description)
	}
	if retrieved.KeyPrefix != apiKey.KeyPrefix {
		t.Errorf("GetByID() keyPrefix = %v, want %v", retrieved.KeyPrefix, apiKey.KeyPrefix)
	}
	if retrieved.CreatedBy != apiKey.CreatedBy {
		t.Errorf("GetByID() createdBy = %v, want %v", retrieved.CreatedBy, apiKey.CreatedBy)
	}

	// Test List
	keys, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(keys) != 1 {
		t.Errorf("List() returned %d keys, want 1", len(keys))
	}

	// Test Update
	apiKey.Role = auth.RoleAdmin
	apiKey.Description = "Updated description"
	if err := store.Update(ctx, apiKey); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	retrieved, err = store.GetByID(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetByID() after Update error = %v", err)
	}
	if retrieved.Role != auth.RoleAdmin {
		t.Errorf("Update() role = %v, want %v", retrieved.Role, auth.RoleAdmin)
	}
	if retrieved.Description != "Updated description" {
		t.Errorf("Update() description = %v, want %v", retrieved.Description, "Updated description")
	}

	// Test UpdateLastUsed
	if err := store.UpdateLastUsed(ctx, apiKey.ID); err != nil {
		t.Fatalf("UpdateLastUsed() error = %v", err)
	}
	retrieved, err = store.GetByID(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetByID() after UpdateLastUsed error = %v", err)
	}
	if retrieved.LastUsedAt == nil {
		t.Error("UpdateLastUsed() should set LastUsedAt")
	}

	// Test Delete
	if err := store.Delete(ctx, apiKey.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = store.GetByID(ctx, apiKey.ID)
	if err != auth.ErrAPIKeyNotFound {
		t.Errorf("GetByID() after delete error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}
}

func TestStore_DuplicateName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Create first key
	key1 := auth.NewAPIKey("test-key", "First key", auth.RoleViewer, "hash1", "dagu_111", "admin")
	if err := store.Create(ctx, key1); err != nil {
		t.Fatalf("Create() first key error = %v", err)
	}

	// Try to create second key with same name
	key2 := auth.NewAPIKey("test-key", "Second key", auth.RoleManager, "hash2", "dagu_222", "admin")
	err = store.Create(ctx, key2)
	if err != auth.ErrAPIKeyAlreadyExists {
		t.Errorf("Create() duplicate name error = %v, want %v", err, auth.ErrAPIKeyAlreadyExists)
	}
}

func TestStore_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test GetByID not found
	_, err = store.GetByID(ctx, "nonexistent-id")
	if err != auth.ErrAPIKeyNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}

	// Test Delete not found
	err = store.Delete(ctx, "nonexistent-id")
	if err != auth.ErrAPIKeyNotFound {
		t.Errorf("Delete() error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}

	// Test Update not found
	key := auth.NewAPIKey("test", "desc", auth.RoleViewer, "hash", "dagu_xxx", "admin")
	err = store.Update(ctx, key)
	if err != auth.ErrAPIKeyNotFound {
		t.Errorf("Update() error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}

	// Test UpdateLastUsed not found
	err = store.UpdateLastUsed(ctx, "nonexistent-id")
	if err != auth.ErrAPIKeyNotFound {
		t.Errorf("UpdateLastUsed() error = %v, want %v", err, auth.ErrAPIKeyNotFound)
	}
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store and add key
	store1, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	apiKey := auth.NewAPIKey("test-key", "Test key", auth.RoleAdmin, "hash", "dagu_xxx", "admin")
	if err := store1.Create(ctx, apiKey); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create new store instance (simulates restart)
	store2, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}

	// Verify key is found after index rebuild
	retrieved, err := store2.GetByID(ctx, apiKey.ID)
	if err != nil {
		t.Fatalf("GetByID() after rebuild error = %v", err)
	}
	if retrieved.Name != apiKey.Name {
		t.Errorf("GetByID() after rebuild name = %v, want %v", retrieved.Name, apiKey.Name)
	}
}

func TestStore_EmptyBaseDir(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Error("New() with empty baseDir should return error")
	}
}

func TestStore_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	apiKey := auth.NewAPIKey("test-key", "Test key", auth.RoleViewer, "hash", "dagu_xxx", "admin")
	if err := store.Create(ctx, apiKey); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, apiKey.ID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("API key file should exist after Create()")
	}

	// Verify file is deleted after Delete
	if err := store.Delete(ctx, apiKey.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("API key file should not exist after Delete()")
	}
}

func TestStore_UpdateName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Create two keys
	key1 := auth.NewAPIKey("key-one", "First key", auth.RoleViewer, "hash1", "dagu_111", "admin")
	key2 := auth.NewAPIKey("key-two", "Second key", auth.RoleViewer, "hash2", "dagu_222", "admin")
	if err := store.Create(ctx, key1); err != nil {
		t.Fatalf("Create() first key error = %v", err)
	}
	if err := store.Create(ctx, key2); err != nil {
		t.Fatalf("Create() second key error = %v", err)
	}

	// Update key1 name to a new unique name
	key1.Name = "key-one-updated"
	if err := store.Update(ctx, key1); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	// Try to update key1 name to key2's name (should fail)
	key1.Name = "key-two"
	err = store.Update(ctx, key1)
	if err != auth.ErrAPIKeyAlreadyExists {
		t.Errorf("Update() to duplicate name error = %v, want %v", err, auth.ErrAPIKeyAlreadyExists)
	}
}
