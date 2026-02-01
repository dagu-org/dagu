package filewebhook

import (
	"context"
	"fmt"
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

func setupStore(t *testing.T) (*Store, string) {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := New(tmpDir)
	require.NoError(t, err)
	return store, tmpDir
}

func setupStoreWithCache(t *testing.T) (*Store, string, *fileutil.Cache[*auth.Webhook]) {
	t.Helper()
	tmpDir := t.TempDir()
	cache := fileutil.NewCache[*auth.Webhook]("webhook_test", 100, time.Hour)
	store, err := New(tmpDir, WithFileCache(cache))
	require.NoError(t, err)
	return store, tmpDir, cache
}

func newWebhook(t *testing.T, dagName string) *auth.Webhook {
	t.Helper()
	wh, err := auth.NewWebhook(dagName, "hash", "dagu_wh_", "admin")
	require.NoError(t, err)
	return wh
}

func TestStore_CRUD(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	// Create
	wh := newWebhook(t, "test-dag")
	require.NoError(t, store.Create(ctx, wh))

	// GetByID
	got, err := store.GetByID(ctx, wh.ID)
	require.NoError(t, err)
	assert.Equal(t, wh.DAGName, got.DAGName)
	assert.True(t, got.Enabled)

	// GetByDAGName
	got, err = store.GetByDAGName(ctx, "test-dag")
	require.NoError(t, err)
	assert.Equal(t, wh.ID, got.ID)

	// List
	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	// Update
	wh.Enabled = false
	require.NoError(t, store.Update(ctx, wh))
	got, _ = store.GetByID(ctx, wh.ID)
	assert.False(t, got.Enabled)

	// UpdateLastUsed
	require.NoError(t, store.UpdateLastUsed(ctx, wh.ID))
	got, _ = store.GetByID(ctx, wh.ID)
	assert.NotNil(t, got.LastUsedAt)

	// Delete
	require.NoError(t, store.Delete(ctx, wh.ID))
	_, err = store.GetByID(ctx, wh.ID)
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
}

func TestStore_DuplicateDAGName(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh1 := newWebhook(t, "same-dag")
	require.NoError(t, store.Create(ctx, wh1))

	wh2 := newWebhook(t, "same-dag")
	err := store.Create(ctx, wh2)
	assert.ErrorIs(t, err, auth.ErrWebhookAlreadyExists)
}

func TestStore_DuplicateID(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh1 := newWebhook(t, "dag-1")
	require.NoError(t, store.Create(ctx, wh1))

	wh2 := &auth.Webhook{ID: wh1.ID, DAGName: "dag-2", TokenHash: "hash"}
	err := store.Create(ctx, wh2)
	assert.ErrorIs(t, err, auth.ErrWebhookAlreadyExists)
}

func TestStore_NotFound(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "missing")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	_, err = store.GetByDAGName(ctx, "missing")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	err = store.Delete(ctx, "missing")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	err = store.DeleteByDAGName(ctx, "missing")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	wh := newWebhook(t, "test")
	err = store.Update(ctx, wh)
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	err = store.UpdateLastUsed(ctx, "missing")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
}

func TestStore_DeleteByDAGName(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh := newWebhook(t, "to-delete")
	require.NoError(t, store.Create(ctx, wh))

	require.NoError(t, store.DeleteByDAGName(ctx, "to-delete"))

	_, err := store.GetByID(ctx, wh.ID)
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
}

func TestStore_RebuildIndex(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	store1, err := New(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	wh := newWebhook(t, "persist-dag")
	require.NoError(t, store1.Create(ctx, wh))

	// New store instance should rebuild index from files
	store2, err := New(tmpDir)
	require.NoError(t, err)

	got, err := store2.GetByID(ctx, wh.ID)
	require.NoError(t, err)
	assert.Equal(t, wh.DAGName, got.DAGName)

	got, err = store2.GetByDAGName(ctx, "persist-dag")
	require.NoError(t, err)
	assert.Equal(t, wh.ID, got.ID)
}

func TestStore_RebuildIndexSkipsInvalidFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	store, err := New(tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	wh := newWebhook(t, "valid-dag")
	require.NoError(t, store.Create(ctx, wh))

	// Add corrupted file, non-JSON file, and directory
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "bad.json"), []byte("{invalid"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "readme.txt"), []byte("text"), 0600))
	require.NoError(t, os.Mkdir(filepath.Join(tmpDir, "subdir.json"), 0750))

	store2, err := New(tmpDir)
	require.NoError(t, err)

	list, err := store2.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "valid-dag", list[0].DAGName)
}

func TestStore_UpdateDAGName(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh := newWebhook(t, "old-name")
	require.NoError(t, store.Create(ctx, wh))

	wh.DAGName = "new-name"
	require.NoError(t, store.Update(ctx, wh))

	_, err := store.GetByDAGName(ctx, "old-name")
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)

	got, err := store.GetByDAGName(ctx, "new-name")
	require.NoError(t, err)
	assert.Equal(t, wh.ID, got.ID)
}

func TestStore_UpdateDAGNameConflict(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh1 := newWebhook(t, "dag-one")
	wh2 := newWebhook(t, "dag-two")
	require.NoError(t, store.Create(ctx, wh1))
	require.NoError(t, store.Create(ctx, wh2))

	wh1.DAGName = "dag-two"
	err := store.Update(ctx, wh1)
	assert.ErrorIs(t, err, auth.ErrWebhookAlreadyExists)
}

func TestStore_InputValidation(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	t.Run("CreateNil", func(t *testing.T) {
		t.Parallel()
		err := store.Create(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("CreateEmptyID", func(t *testing.T) {
		t.Parallel()
		err := store.Create(ctx, &auth.Webhook{DAGName: "dag"})
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookID)
	})

	t.Run("CreateEmptyDAGName", func(t *testing.T) {
		t.Parallel()
		err := store.Create(ctx, &auth.Webhook{ID: "id"})
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookDAGName)
	})

	t.Run("UpdateNil", func(t *testing.T) {
		t.Parallel()
		err := store.Update(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("GetByIDEmpty", func(t *testing.T) {
		t.Parallel()
		_, err := store.GetByID(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookID)
	})

	t.Run("GetByDAGNameEmpty", func(t *testing.T) {
		t.Parallel()
		_, err := store.GetByDAGName(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookDAGName)
	})

	t.Run("DeleteEmpty", func(t *testing.T) {
		t.Parallel()
		err := store.Delete(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookID)
	})

	t.Run("DeleteByDAGNameEmpty", func(t *testing.T) {
		t.Parallel()
		err := store.DeleteByDAGName(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookDAGName)
	})

	t.Run("UpdateLastUsedEmpty", func(t *testing.T) {
		t.Parallel()
		err := store.UpdateLastUsed(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidWebhookID)
	})
}

func TestStore_CorruptedFile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name string
		op   func(*Store, *auth.Webhook) error
	}{
		{"GetByID", func(s *Store, wh *auth.Webhook) error {
			_, err := s.GetByID(ctx, wh.ID)
			return err
		}},
		{"Update", func(s *Store, wh *auth.Webhook) error {
			return s.Update(ctx, wh)
		}},
		{"Delete", func(s *Store, wh *auth.Webhook) error {
			return s.Delete(ctx, wh.ID)
		}},
		{"UpdateLastUsed", func(s *Store, wh *auth.Webhook) error {
			return s.UpdateLastUsed(ctx, wh.ID)
		}},
		{"List", func(s *Store, _ *auth.Webhook) error {
			_, err := s.List(ctx)
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			store, tmpDir := setupStore(t)
			wh := newWebhook(t, "dag-"+tc.name)
			require.NoError(t, store.Create(ctx, wh))

			// Corrupt the file
			filePath := filepath.Join(tmpDir, wh.ID+".json")
			require.NoError(t, os.WriteFile(filePath, []byte("{invalid"), 0600))

			err := tc.op(store, wh)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to")
		})
	}
}

func TestStore_ExternalFileDeletion(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	t.Run("GetByID", func(t *testing.T) {
		t.Parallel()
		store, tmpDir := setupStore(t)
		wh := newWebhook(t, "dag")
		require.NoError(t, store.Create(ctx, wh))
		require.NoError(t, os.Remove(filepath.Join(tmpDir, wh.ID+".json")))

		_, err := store.GetByID(ctx, wh.ID)
		assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		t.Parallel()
		store, tmpDir := setupStore(t)
		wh := newWebhook(t, "dag")
		require.NoError(t, store.Create(ctx, wh))
		require.NoError(t, os.Remove(filepath.Join(tmpDir, wh.ID+".json")))

		// Should succeed (file already gone)
		require.NoError(t, store.Delete(ctx, wh.ID))
	})

	t.Run("UpdateLastUsed", func(t *testing.T) {
		t.Parallel()
		store, tmpDir := setupStore(t)
		wh := newWebhook(t, "dag")
		require.NoError(t, store.Create(ctx, wh))
		require.NoError(t, os.Remove(filepath.Join(tmpDir, wh.ID+".json")))

		err := store.UpdateLastUsed(ctx, wh.ID)
		assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
	})
}

func TestStore_WriteError(t *testing.T) {
	// Skip if running as root since permission-based write failures cannot be reliably tested
	if os.Getuid() == 0 {
		t.Skip("cannot test permission errors as root")
	}

	t.Parallel()
	ctx := context.Background()

	tests := []struct {
		name   string
		setup  func(*Store, context.Context) *auth.Webhook
		action func(*Store, context.Context, *auth.Webhook) error
	}{
		{
			name:  "Create",
			setup: func(_ *Store, _ context.Context) *auth.Webhook { return newWebhook(t, "dag") },
			action: func(s *Store, ctx context.Context, wh *auth.Webhook) error {
				return s.Create(ctx, wh)
			},
		},
		{
			name: "Update",
			setup: func(s *Store, ctx context.Context) *auth.Webhook {
				wh := newWebhook(t, "dag")
				_ = s.Create(ctx, wh)
				return wh
			},
			action: func(s *Store, ctx context.Context, wh *auth.Webhook) error {
				wh.Enabled = false
				return s.Update(ctx, wh)
			},
		},
		{
			name: "UpdateLastUsed",
			setup: func(s *Store, ctx context.Context) *auth.Webhook {
				wh := newWebhook(t, "dag")
				_ = s.Create(ctx, wh)
				return wh
			},
			action: func(s *Store, ctx context.Context, wh *auth.Webhook) error {
				return s.UpdateLastUsed(ctx, wh.ID)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpDir := t.TempDir()
			store, err := New(tmpDir)
			require.NoError(t, err)

			wh := tc.setup(store, ctx)

			// Make directory read-only
			require.NoError(t, os.Chmod(tmpDir, 0500))
			defer func() { _ = os.Chmod(tmpDir, 0750) }()

			err = tc.action(store, ctx, wh)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "failed to create temp file")
		})
	}
}

func TestStore_Cache(t *testing.T) {
	t.Parallel()
	store, tmpDir, cache := setupStoreWithCache(t)
	ctx := context.Background()

	wh := newWebhook(t, "cached-dag")
	require.NoError(t, store.Create(ctx, wh))

	filePath := filepath.Join(tmpDir, wh.ID+".json")

	// Read populates cache
	_, err := store.GetByID(ctx, wh.ID)
	require.NoError(t, err)
	_, found := cache.Load(filePath)
	assert.True(t, found)

	// Update invalidates cache
	wh.Enabled = false
	require.NoError(t, store.Update(ctx, wh))
	_, found = cache.Load(filePath)
	assert.False(t, found)

	// Populate again
	_, _ = store.GetByID(ctx, wh.ID)

	// Delete invalidates cache
	require.NoError(t, store.Delete(ctx, wh.ID))
	_, found = cache.Load(filePath)
	assert.False(t, found)
}

func TestStore_CacheWithExternalDeletion(t *testing.T) {
	t.Parallel()
	store, tmpDir, _ := setupStoreWithCache(t)
	ctx := context.Background()

	wh := newWebhook(t, "cache-delete")
	require.NoError(t, store.Create(ctx, wh))

	// Populate cache
	_, err := store.GetByID(ctx, wh.ID)
	require.NoError(t, err)

	// Delete file externally
	require.NoError(t, os.Remove(filepath.Join(tmpDir, wh.ID+".json")))

	// Should detect file is gone
	_, err = store.GetByID(ctx, wh.ID)
	assert.ErrorIs(t, err, auth.ErrWebhookNotFound)
}

func TestStore_FilePermissions(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "webhooks")

	store, err := New(subDir)
	require.NoError(t, err)

	// Directory permission
	info, err := os.Stat(subDir)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0750), info.Mode().Perm())

	// File permission
	wh := newWebhook(t, "perm-test")
	require.NoError(t, store.Create(context.Background(), wh))

	info, err = os.Stat(filepath.Join(subDir, wh.ID+".json"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestStore_Concurrent(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	const n = 10
	var wg sync.WaitGroup
	errs := make(chan error, n*2)

	// Concurrent creates
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			wh := newWebhook(t, fmt.Sprintf("dag-%d", i))
			if err := store.Create(ctx, wh); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := store.List(ctx); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}

	list, err := store.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, n)
}

func TestStore_ConcurrentSameDAG(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	const n = 10
	var wg sync.WaitGroup
	var successCount, dupCount int32
	var mu sync.Mutex

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			wh := newWebhook(t, "same-dag")
			err := store.Create(ctx, wh)
			mu.Lock()
			switch err {
			case nil:
				successCount++
			case auth.ErrWebhookAlreadyExists:
				dupCount++
			}
			mu.Unlock()
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(1), successCount)
	assert.Equal(t, int32(n-1), dupCount)
}

func TestStore_EmptyBaseDir(t *testing.T) {
	t.Parallel()
	_, err := New("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "baseDir cannot be empty")
}

func TestStore_UpdateLastUsedPreservesFields(t *testing.T) {
	t.Parallel()
	store, _ := setupStore(t)
	ctx := context.Background()

	wh, _ := auth.NewWebhook("dag", "secret-hash", "dagu_wh_", "creator")
	wh.Enabled = false
	require.NoError(t, store.Create(ctx, wh))

	require.NoError(t, store.UpdateLastUsed(ctx, wh.ID))

	got, _ := store.GetByID(ctx, wh.ID)
	assert.Equal(t, wh.DAGName, got.DAGName)
	assert.Equal(t, wh.TokenHash, got.TokenHash)
	assert.Equal(t, wh.CreatedBy, got.CreatedBy)
	assert.False(t, got.Enabled)
	assert.NotNil(t, got.LastUsedAt)
}
