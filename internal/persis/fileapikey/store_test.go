package fileapikey

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test helper functions

func setupTestStore(t *testing.T) (*Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	require.NoError(t, err)
	return store, tmpDir
}

func setupTestStoreWithCache(t *testing.T) (*Store, string, *fileutil.Cache[*auth.APIKey]) {
	t.Helper()
	tmpDir := t.TempDir()
	cache := fileutil.NewCache[*auth.APIKey]("api_key_test", 100, time.Hour)
	store, err := New(tmpDir, WithFileCache(cache))
	require.NoError(t, err)
	return store, tmpDir, cache
}

func createTestAPIKey(t *testing.T, name string) *auth.APIKey {
	t.Helper()
	key, err := auth.NewAPIKey(name, "Test key", auth.RoleViewer, "hash", "dagu_xxx", "admin")
	require.NoError(t, err)
	return key
}

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

func TestStore_InputValidation(t *testing.T) {
	t.Run("Create", func(t *testing.T) {
		t.Run("NilKey", func(t *testing.T) {
			store, _ := setupTestStore(t)
			err := store.Create(context.Background(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "API key cannot be nil")
		})

		t.Run("EmptyID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			key := &auth.APIKey{
				ID:      "",
				Name:    "test-key",
				KeyHash: "hash",
			}
			err := store.Create(context.Background(), key)
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyID)
		})

		t.Run("EmptyName", func(t *testing.T) {
			store, _ := setupTestStore(t)
			key := &auth.APIKey{
				ID:      "some-id",
				Name:    "",
				KeyHash: "hash",
			}
			err := store.Create(context.Background(), key)
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyName)
		})

		t.Run("DuplicateID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			ctx := context.Background()

			key1 := createTestAPIKey(t, "key-one")
			err := store.Create(ctx, key1)
			require.NoError(t, err)

			// Create another key with the same ID but different name
			key2 := &auth.APIKey{
				ID:      key1.ID, // Same ID
				Name:    "key-two",
				KeyHash: "hash2",
			}
			err = store.Create(ctx, key2)
			assert.ErrorIs(t, err, auth.ErrAPIKeyAlreadyExists)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("NilKey", func(t *testing.T) {
			store, _ := setupTestStore(t)
			err := store.Update(context.Background(), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "API key cannot be nil")
		})

		t.Run("EmptyID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			key := &auth.APIKey{
				ID:      "",
				Name:    "test-key",
				KeyHash: "hash",
			}
			err := store.Update(context.Background(), key)
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyID)
		})

		t.Run("EmptyName", func(t *testing.T) {
			store, _ := setupTestStore(t)
			ctx := context.Background()

			// First create a valid key
			key := createTestAPIKey(t, "original-name")
			err := store.Create(ctx, key)
			require.NoError(t, err)

			// Try to update with empty name
			key.Name = ""
			err = store.Update(ctx, key)
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyName)
		})
	})

	t.Run("GetByID", func(t *testing.T) {
		t.Run("EmptyID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			_, err := store.GetByID(context.Background(), "")
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyID)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("EmptyID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			err := store.Delete(context.Background(), "")
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyID)
		})
	})

	t.Run("UpdateLastUsed", func(t *testing.T) {
		t.Run("EmptyID", func(t *testing.T) {
			store, _ := setupTestStore(t)
			err := store.UpdateLastUsed(context.Background(), "")
			assert.ErrorIs(t, err, auth.ErrInvalidAPIKeyID)
		})
	})
}

func TestStore_FileCache(t *testing.T) {
	t.Run("CacheHit", func(t *testing.T) {
		store, tmpDir, cache := setupTestStoreWithCache(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "cached-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// First read should populate cache
		retrieved1, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, key.Name, retrieved1.Name)

		// Verify cache has the entry
		filePath := filepath.Join(tmpDir, key.ID+".json")
		cachedKey, found := cache.Load(filePath)
		assert.True(t, found, "Cache should contain the API key")
		assert.Equal(t, key.Name, cachedKey.Name)

		// Second read should use cache (verified by cache hit)
		retrieved2, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, key.Name, retrieved2.Name)
	})

	t.Run("InvalidationOnUpdate", func(t *testing.T) {
		store, tmpDir, cache := setupTestStoreWithCache(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "update-cache-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Read to populate cache
		_, err = store.GetByID(ctx, key.ID)
		require.NoError(t, err)

		filePath := filepath.Join(tmpDir, key.ID+".json")
		_, found := cache.Load(filePath)
		assert.True(t, found, "Cache should contain the API key before update")

		// Update should invalidate cache
		key.Description = "Updated description"
		err = store.Update(ctx, key)
		require.NoError(t, err)

		// Cache should be invalidated
		_, found = cache.Load(filePath)
		assert.False(t, found, "Cache should be invalidated after update")

		// Next read should get updated data
		retrieved, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, "Updated description", retrieved.Description)
	})

	t.Run("InvalidationOnDelete", func(t *testing.T) {
		store, tmpDir, cache := setupTestStoreWithCache(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "delete-cache-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Read to populate cache
		_, err = store.GetByID(ctx, key.ID)
		require.NoError(t, err)

		filePath := filepath.Join(tmpDir, key.ID+".json")
		_, found := cache.Load(filePath)
		assert.True(t, found, "Cache should contain the API key before delete")

		// Delete should invalidate cache
		err = store.Delete(ctx, key.ID)
		require.NoError(t, err)

		// Cache should be invalidated
		_, found = cache.Load(filePath)
		assert.False(t, found, "Cache should be invalidated after delete")
	})

	t.Run("InvalidationOnUpdateLastUsed", func(t *testing.T) {
		store, tmpDir, cache := setupTestStoreWithCache(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "lastused-cache-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Read to populate cache
		_, err = store.GetByID(ctx, key.ID)
		require.NoError(t, err)

		filePath := filepath.Join(tmpDir, key.ID+".json")
		_, found := cache.Load(filePath)
		assert.True(t, found, "Cache should contain the API key before UpdateLastUsed")

		// UpdateLastUsed should invalidate cache
		err = store.UpdateLastUsed(ctx, key.ID)
		require.NoError(t, err)

		// Cache should be invalidated
		_, found = cache.Load(filePath)
		assert.False(t, found, "Cache should be invalidated after UpdateLastUsed")
	})

	t.Run("StaleDetection", func(t *testing.T) {
		store, tmpDir, _ := setupTestStoreWithCache(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "stale-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Read to populate cache
		_, err = store.GetByID(ctx, key.ID)
		require.NoError(t, err)

		filePath := filepath.Join(tmpDir, key.ID+".json")
		time.Sleep(10 * time.Millisecond)

		modifiedData := []byte(`{
  "id": "` + key.ID + `",
  "name": "stale-key",
  "description": "Externally modified",
  "role": "viewer",
  "key_hash": "hash",
  "key_prefix": "dagu_xxx",
  "created_at": "2024-01-01T00:00:00Z",
  "updated_at": "2024-01-01T00:00:00Z",
  "created_by": "admin"
}`)
		err = os.WriteFile(filePath, modifiedData, 0600)
		require.NoError(t, err)

		// Read should detect stale cache and reload
		retrieved, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, "Externally modified", retrieved.Description)
	})
}

func TestStore_FileSystemEdgeCases(t *testing.T) {
	t.Run("FilePermissions", func(t *testing.T) {
		store, tmpDir := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "perm-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		filePath := filepath.Join(tmpDir, key.ID+".json")
		info, err := os.Stat(filePath)
		require.NoError(t, err)

		// Check file permissions (0600 = -rw-------)
		// Note: On some systems, umask might affect this
		perm := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0600), perm, "API key file should have 0600 permissions")
	})

	t.Run("DirectoryPermissions", func(t *testing.T) {
		tmpDir := t.TempDir()
		subDir := filepath.Join(tmpDir, "apikeys")

		// Create store in a new subdirectory (should create it)
		_, err := New(subDir)
		require.NoError(t, err)

		info, err := os.Stat(subDir)
		require.NoError(t, err)

		// Check directory permissions (0750 = drwxr-x---)
		perm := info.Mode().Perm()
		assert.Equal(t, os.FileMode(0750), perm, "API key directory should have 0750 permissions")
	})

	t.Run("ExternalFileDeletion", func(t *testing.T) {
		store, tmpDir := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "external-delete-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Delete file externally
		filePath := filepath.Join(tmpDir, key.ID+".json")
		err = os.Remove(filePath)
		require.NoError(t, err)

		// GetByID should return ErrAPIKeyNotFound
		_, err = store.GetByID(ctx, key.ID)
		assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
	})

	t.Run("DeleteAlreadyRemovedFile", func(t *testing.T) {
		store, tmpDir := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "already-removed-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Delete file externally first
		filePath := filepath.Join(tmpDir, key.ID+".json")
		err = os.Remove(filePath)
		require.NoError(t, err)

		// Store's Delete should still succeed (graceful handling)
		err = store.Delete(ctx, key.ID)
		require.NoError(t, err)

		// Key should not be in index anymore
		_, err = store.GetByID(ctx, key.ID)
		assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
	})

	t.Run("DeleteWithOrphanedIndex", func(t *testing.T) {
		store, tmpDir := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "orphaned-index-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		// Remove file but keep index entry (simulating crash during delete)
		filePath := filepath.Join(tmpDir, key.ID+".json")
		err = os.Remove(filePath)
		require.NoError(t, err)

		// Delete should clean up orphaned index entry
		err = store.Delete(ctx, key.ID)
		require.NoError(t, err)

		// Verify key is completely gone from index
		keys, err := store.List(ctx)
		require.NoError(t, err)
		for _, k := range keys {
			assert.NotEqual(t, key.ID, k.ID, "Orphaned key should be removed from index")
		}
	})

	t.Run("RebuildIndexSkipsNonJSONFiles", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create a non-JSON file in the directory
		nonJSONPath := filepath.Join(tmpDir, "readme.txt")
		err := os.WriteFile(nonJSONPath, []byte("This is not JSON"), 0600)
		require.NoError(t, err)

		// Create a subdirectory (should be skipped)
		subDir := filepath.Join(tmpDir, "subdir.json") // Named like JSON but is a directory
		err = os.Mkdir(subDir, 0750)
		require.NoError(t, err)

		// Create store - should not fail on non-JSON files
		store, err := New(tmpDir)
		require.NoError(t, err)

		// Store should be empty (no valid API keys)
		keys, err := store.List(context.Background())
		require.NoError(t, err)
		assert.Empty(t, keys)
	})
}

func TestStore_ConcurrentEdgeCases(t *testing.T) {
	t.Run("UpdateSameKey", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "concurrent-update-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		const numGoroutines = 20
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// Concurrent updates to the same key
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				keyCopy := *key
				keyCopy.Description = "Updated by goroutine " + string(rune('A'+idx))
				if err := store.Update(ctx, &keyCopy); err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// Collect any errors
		var errs []error
		for err := range errChan {
			errs = append(errs, err)
		}
		assert.Empty(t, errs, "No errors should occur during concurrent updates")

		// Key should still be valid and retrievable
		retrieved, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.NotEmpty(t, retrieved.Description)
	})

	t.Run("CreateSameName", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		const numGoroutines = 10
		var wg sync.WaitGroup
		successCount := make(chan int, numGoroutines)
		duplicateCount := make(chan int, numGoroutines)

		// Concurrent creates with the same name
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				key := createTestAPIKey(t, "same-name-key")
				err := store.Create(ctx, key)
				switch err {
				case nil:
					successCount <- 1
				case auth.ErrAPIKeyAlreadyExists:
					duplicateCount <- 1
				}
			}()
		}

		wg.Wait()
		close(successCount)
		close(duplicateCount)

		successes := 0
		for range successCount {
			successes++
		}

		duplicates := 0
		for range duplicateCount {
			duplicates++
		}

		// Exactly one should succeed, rest should fail with duplicate error
		assert.Equal(t, 1, successes, "Exactly one create should succeed")
		assert.Equal(t, numGoroutines-1, duplicates, "Rest should fail with ErrAPIKeyAlreadyExists")

		// Verify only one key exists
		keys, err := store.List(ctx)
		require.NoError(t, err)
		assert.Len(t, keys, 1)
	})

	t.Run("DeleteWhileReading", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "delete-while-read-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		const numReaders = 10
		var wg sync.WaitGroup
		startChan := make(chan struct{})

		// Start readers
		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-startChan
				// Multiple reads - some might fail after delete
				for j := 0; j < 5; j++ {
					_, err := store.GetByID(ctx, key.ID)
					// Either success or not found is acceptable
					if err != nil && err != auth.ErrAPIKeyNotFound {
						t.Errorf("Unexpected error: %v", err)
					}
				}
			}()
		}

		// Start delete
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-startChan
			_ = store.Delete(ctx, key.ID)
		}()

		// Start all goroutines simultaneously
		close(startChan)
		wg.Wait()

		// After all operations, key should not exist
		_, err = store.GetByID(ctx, key.ID)
		assert.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
	})

	t.Run("ConcurrentUpdateLastUsed", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		key := createTestAPIKey(t, "concurrent-lastused-key")
		err := store.Create(ctx, key)
		require.NoError(t, err)

		const numGoroutines = 20
		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if err := store.UpdateLastUsed(ctx, key.ID); err != nil {
					errChan <- err
				}
			}()
		}

		wg.Wait()
		close(errChan)

		var errs []error
		for err := range errChan {
			errs = append(errs, err)
		}
		assert.Empty(t, errs, "No errors should occur during concurrent UpdateLastUsed")

		// LastUsedAt should be set
		retrieved, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.NotNil(t, retrieved.LastUsedAt)
	})
}

func TestStore_ListEdgeCases(t *testing.T) {
	t.Run("EmptyStore", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		keys, err := store.List(ctx)
		require.NoError(t, err)
		assert.NotNil(t, keys, "List should return non-nil slice")
		assert.Empty(t, keys, "List should return empty slice for empty store")
	})

	t.Run("ExternalDeletion", func(t *testing.T) {
		store, tmpDir := setupTestStore(t)
		ctx := context.Background()

		// Create multiple keys
		key1 := createTestAPIKey(t, "list-key-1")
		key2 := createTestAPIKey(t, "list-key-2")
		key3 := createTestAPIKey(t, "list-key-3")

		require.NoError(t, store.Create(ctx, key1))
		require.NoError(t, store.Create(ctx, key2))
		require.NoError(t, store.Create(ctx, key3))

		// Delete one file externally
		filePath := filepath.Join(tmpDir, key2.ID+".json")
		err := os.Remove(filePath)
		require.NoError(t, err)

		// List should gracefully skip the missing file
		keys, err := store.List(ctx)
		require.NoError(t, err)
		assert.Len(t, keys, 2, "List should return 2 keys (skipping deleted one)")

		// Verify the remaining keys
		keyNames := make(map[string]bool)
		for _, k := range keys {
			keyNames[k.Name] = true
		}
		assert.True(t, keyNames["list-key-1"])
		assert.True(t, keyNames["list-key-3"])
		assert.False(t, keyNames["list-key-2"])
	})

	t.Run("LargeNumberOfKeys", func(t *testing.T) {
		store, _ := setupTestStore(t)
		ctx := context.Background()

		const numKeys = 100

		// Create many keys
		for i := 0; i < numKeys; i++ {
			key, err := auth.NewAPIKey(
				"bulk-key-"+string(rune('0'+(i/100)%10))+string(rune('0'+(i/10)%10))+string(rune('0'+i%10)),
				"Bulk key",
				auth.RoleViewer,
				"hash",
				"dagu_",
				"admin",
			)
			require.NoError(t, err)
			err = store.Create(ctx, key)
			require.NoError(t, err)
		}

		// List all keys
		keys, err := store.List(ctx)
		require.NoError(t, err)
		assert.Len(t, keys, numKeys)
	})
}

func TestStore_UpdateLastUsedPreservesFields(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	// Create key with specific values
	key, err := auth.NewAPIKey(
		"preserve-fields-key",
		"Original description",
		auth.RoleManager,
		"original-hash",
		"dagu_pre",
		"creator-user",
	)
	require.NoError(t, err)
	err = store.Create(ctx, key)
	require.NoError(t, err)

	// Record original values
	originalName := key.Name
	originalDescription := key.Description
	originalRole := key.Role
	originalKeyHash := key.KeyHash
	originalKeyPrefix := key.KeyPrefix
	originalCreatedBy := key.CreatedBy
	originalCreatedAt := key.CreatedAt

	// Update LastUsed
	err = store.UpdateLastUsed(ctx, key.ID)
	require.NoError(t, err)

	// Retrieve and verify all fields preserved
	retrieved, err := store.GetByID(ctx, key.ID)
	require.NoError(t, err)

	assert.Equal(t, originalName, retrieved.Name, "Name should be preserved")
	assert.Equal(t, originalDescription, retrieved.Description, "Description should be preserved")
	assert.Equal(t, originalRole, retrieved.Role, "Role should be preserved")
	assert.Equal(t, originalKeyHash, retrieved.KeyHash, "KeyHash should be preserved")
	assert.Equal(t, originalKeyPrefix, retrieved.KeyPrefix, "KeyPrefix should be preserved")
	assert.Equal(t, originalCreatedBy, retrieved.CreatedBy, "CreatedBy should be preserved")
	assert.Equal(t, originalCreatedAt.Unix(), retrieved.CreatedAt.Unix(), "CreatedAt should be preserved")
	assert.NotNil(t, retrieved.LastUsedAt, "LastUsedAt should be set")
}

func TestStore_UpdateNameToSameName(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "same-name-update")
	err := store.Create(ctx, key)
	require.NoError(t, err)

	// Update with the same name (should succeed)
	key.Description = "Updated description"
	err = store.Update(ctx, key)
	require.NoError(t, err)

	// Verify update worked
	retrieved, err := store.GetByID(ctx, key.ID)
	require.NoError(t, err)
	assert.Equal(t, "same-name-update", retrieved.Name)
	assert.Equal(t, "Updated description", retrieved.Description)
}

func TestStore_MultipleUpdateNameChanges(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	key := createTestAPIKey(t, "original-name")
	err := store.Create(ctx, key)
	require.NoError(t, err)

	// Change name multiple times
	names := []string{"first-update", "second-update", "third-update", "final-name"}
	for _, name := range names {
		key.Name = name
		err = store.Update(ctx, key)
		require.NoError(t, err)

		// Verify the name was updated
		retrieved, err := store.GetByID(ctx, key.ID)
		require.NoError(t, err)
		assert.Equal(t, name, retrieved.Name)
	}

	// Verify old names are available for new keys
	for _, oldName := range []string{"original-name", "first-update", "second-update", "third-update"} {
		newKey := createTestAPIKey(t, oldName)
		err = store.Create(ctx, newKey)
		require.NoError(t, err, "Should be able to create key with previously used name: %s", oldName)
	}
}

func TestStore_ErrorWrapping(t *testing.T) {
	t.Run("CreateDirectoryError", func(t *testing.T) {
		// Try to create store in an invalid path
		_, err := New("/nonexistent/deeply/nested/path/that/cannot/exist")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "fileapikey:")
	})

	t.Run("EmptyBaseDirError", func(t *testing.T) {
		_, err := New("")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "baseDir cannot be empty")
	})
}
