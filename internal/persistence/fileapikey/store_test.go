// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileapikey

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store
	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test Create
	apiKey, err := auth.NewAPIKey("test-key", "Test API key", auth.RoleManager, "hashedsecret", "dagu_xxx", "admin-user-id")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, apiKey)
	require.NoError(t, err, "Create() failed")

	// Test GetByID
	retrieved, err := store.GetByID(ctx, apiKey.ID)
	require.NoError(t, err, "GetByID() failed")
	assert.Equal(t, apiKey.Name, retrieved.Name)
	assert.Equal(t, apiKey.Role, retrieved.Role)
	assert.Equal(t, apiKey.Description, retrieved.Description)
	assert.Equal(t, apiKey.KeyPrefix, retrieved.KeyPrefix)
	assert.Equal(t, apiKey.CreatedBy, retrieved.CreatedBy)

	// Test List
	keys, err := store.List(ctx)
	require.NoError(t, err, "List() failed")
	assert.Len(t, keys, 1)

	// Test Update
	apiKey.Role = auth.RoleAdmin
	apiKey.Description = "Updated description"
	err = store.Update(ctx, apiKey)
	require.NoError(t, err, "Update() failed")
	retrieved, err = store.GetByID(ctx, apiKey.ID)
	require.NoError(t, err, "GetByID() after Update failed")
	assert.Equal(t, auth.RoleAdmin, retrieved.Role)
	assert.Equal(t, "Updated description", retrieved.Description)

	// Test UpdateLastUsed
	err = store.UpdateLastUsed(ctx, apiKey.ID)
	require.NoError(t, err, "UpdateLastUsed() failed")
	retrieved, err = store.GetByID(ctx, apiKey.ID)
	require.NoError(t, err, "GetByID() after UpdateLastUsed failed")
	assert.NotNil(t, retrieved.LastUsedAt, "UpdateLastUsed() should set LastUsedAt")

	// Test Delete
	err = store.Delete(ctx, apiKey.ID)
	require.NoError(t, err, "Delete() failed")
	_, err = store.GetByID(ctx, apiKey.ID)
	assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestStore_DuplicateName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Create first key
	key1, err := auth.NewAPIKey("test-key", "First key", auth.RoleViewer, "hash1", "dagu_111", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, key1)
	require.NoError(t, err, "Create() first key failed")

	// Try to create second key with same name
	key2, err := auth.NewAPIKey("test-key", "Second key", auth.RoleManager, "hash2", "dagu_222", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, key2)
	assert.ErrorIs(t, err, auth.ErrAPIKeyAlreadyExists)
}

func TestStore_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test GetByID not found
	_, err = store.GetByID(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)

	// Test Delete not found
	err = store.Delete(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)

	// Test Update not found
	key, err := auth.NewAPIKey("test", "desc", auth.RoleViewer, "hash", "dagu_xxx", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Update(ctx, key)
	assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)

	// Test UpdateLastUsed not found
	err = store.UpdateLastUsed(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store and add key
	store1, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	apiKey, err := auth.NewAPIKey("test-key", "Test key", auth.RoleAdmin, "hash", "dagu_xxx", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store1.Create(ctx, apiKey)
	require.NoError(t, err, "Create() failed")

	// Create new store instance (simulates restart)
	store2, err := New(tmpDir)
	require.NoError(t, err, "failed to create second store")

	// Verify key is found after index rebuild
	retrieved, err := store2.GetByID(ctx, apiKey.ID)
	require.NoError(t, err, "GetByID() after rebuild failed")
	assert.Equal(t, apiKey.Name, retrieved.Name)
}

func TestStore_EmptyBaseDir(t *testing.T) {
	_, err := New("")
	assert.Error(t, err, "New() with empty baseDir should return error")
}

func TestStore_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	apiKey, err := auth.NewAPIKey("test-key", "Test key", auth.RoleViewer, "hash", "dagu_xxx", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, apiKey)
	require.NoError(t, err, "Create() failed")

	// Verify file exists
	filePath := filepath.Join(tmpDir, apiKey.ID+".json")
	_, err = os.Stat(filePath)
	assert.False(t, os.IsNotExist(err), "API key file should exist after Create()")

	// Verify file is deleted after Delete
	err = store.Delete(ctx, apiKey.ID)
	require.NoError(t, err, "Delete() failed")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "API key file should not exist after Delete()")
}

func TestStore_UpdateName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileapikey-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Create two keys
	key1, err := auth.NewAPIKey("key-one", "First key", auth.RoleViewer, "hash1", "dagu_111", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	key2, err := auth.NewAPIKey("key-two", "Second key", auth.RoleViewer, "hash2", "dagu_222", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, key1)
	require.NoError(t, err, "Create() first key failed")
	err = store.Create(ctx, key2)
	require.NoError(t, err, "Create() second key failed")

	// Update key1 name to a new unique name
	key1.Name = "key-one-updated"
	err = store.Update(ctx, key1)
	require.NoError(t, err, "Update() failed")

	// Try to update key1 name to key2's name (should fail)
	key1.Name = "key-two"
	err = store.Update(ctx, key1)
	assert.ErrorIs(t, err, auth.ErrAPIKeyAlreadyExists)
}

func TestStore_RebuildIndexWithCorruptedFile(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileapikey-corrupt-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create a store first to initialize the directory
	store, err := New(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	// Create a valid key
	validKey, err := auth.NewAPIKey("valid-key", "Valid key", auth.RoleViewer, "hash123", "dagu_val", "admin")
	require.NoError(t, err, "NewAPIKey() failed")
	err = store.Create(ctx, validKey)
	require.NoError(t, err)

	// Write a corrupted JSON file directly to the directory
	corruptedFilePath := filepath.Join(tmpDir, "corrupted-key.json")
	err = os.WriteFile(corruptedFilePath, []byte("{ invalid json content"), 0600)
	require.NoError(t, err)

	// Create a new store instance to trigger rebuildIndex
	store2, err := New(tmpDir)
	require.NoError(t, err, "Store should be created even with corrupted files")

	// The valid key should still be accessible
	keys, err := store2.List(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, 1, "Should have 1 valid key despite corrupted file")
	assert.Equal(t, "valid-key", keys[0].Name)
}

func TestStore_ConcurrentOperations(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileapikey-concurrent-test-*")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	const numGoroutines = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*3) // Create, List, Delete operations

	// Concurrent creates
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			key, err := auth.NewAPIKey(
				"concurrent-key-"+string(rune('a'+idx)),
				"Concurrent key",
				auth.RoleViewer,
				"hash"+string(rune('a'+idx)),
				"dagu_"+string(rune('a'+idx)),
				"admin",
			)
			if err != nil {
				errors <- err
				return
			}
			if err := store.Create(ctx, key); err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.List(ctx); err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// Check for errors
	var errs []error
	for err := range errors {
		errs = append(errs, err)
	}
	assert.Empty(t, errs, "No errors should occur during concurrent operations")

	// Verify all keys were created
	keys, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, numGoroutines)
}
