package fileuser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStore_CRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store
	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test Create
	user := auth.NewUser("testuser", "hashedpassword", auth.RoleManager)
	err = store.Create(ctx, user)
	require.NoError(t, err, "Create() failed")

	// Test GetByID
	retrieved, err := store.GetByID(ctx, user.ID)
	require.NoError(t, err, "GetByID() failed")
	assert.Equal(t, user.Username, retrieved.Username)
	assert.Equal(t, user.Role, retrieved.Role)

	// Test GetByUsername
	retrieved, err = store.GetByUsername(ctx, user.Username)
	require.NoError(t, err, "GetByUsername() failed")
	assert.Equal(t, user.ID, retrieved.ID)

	// Test List
	users, err := store.List(ctx)
	require.NoError(t, err, "List() failed")
	assert.Len(t, users, 1)

	// Test Count
	count, err := store.Count(ctx)
	require.NoError(t, err, "Count() failed")
	assert.Equal(t, int64(1), count)

	// Test Update
	user.Role = auth.RoleAdmin
	err = store.Update(ctx, user)
	require.NoError(t, err, "Update() failed")
	retrieved, err = store.GetByID(ctx, user.ID)
	require.NoError(t, err, "GetByID() after Update failed")
	assert.Equal(t, auth.RoleAdmin, retrieved.Role)

	// Test Delete
	err = store.Delete(ctx, user.ID)
	require.NoError(t, err, "Delete() failed")
	_, err = store.GetByID(ctx, user.ID)
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestStore_DuplicateUsername(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Create first user
	user1 := auth.NewUser("testuser", "hash1", auth.RoleViewer)
	err = store.Create(ctx, user1)
	require.NoError(t, err, "Create() first user failed")

	// Try to create second user with same username
	user2 := auth.NewUser("testuser", "hash2", auth.RoleManager)
	err = store.Create(ctx, user2)
	assert.ErrorIs(t, err, auth.ErrUserAlreadyExists)
}

func TestStore_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Test GetByID not found
	_, err = store.GetByID(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, auth.ErrUserNotFound)

	// Test GetByUsername not found
	_, err = store.GetByUsername(ctx, "nonexistent-user")
	assert.ErrorIs(t, err, auth.ErrUserNotFound)

	// Test Delete not found
	err = store.Delete(ctx, "nonexistent-id")
	assert.ErrorIs(t, err, auth.ErrUserNotFound)

	// Test Update not found
	user := auth.NewUser("test", "hash", auth.RoleViewer)
	err = store.Update(ctx, user)
	assert.ErrorIs(t, err, auth.ErrUserNotFound)
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store and add user
	store1, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	user := auth.NewUser("testuser", "hash", auth.RoleAdmin)
	err = store1.Create(ctx, user)
	require.NoError(t, err, "Create() failed")

	// Create new store instance (simulates restart)
	store2, err := New(tmpDir)
	require.NoError(t, err, "failed to create second store")

	// Verify user is found after index rebuild
	retrieved, err := store2.GetByUsername(ctx, "testuser")
	require.NoError(t, err, "GetByUsername() after rebuild failed")
	assert.Equal(t, user.ID, retrieved.ID)
}

func TestStore_EmptyBaseDir(t *testing.T) {
	_, err := New("")
	assert.Error(t, err, "New() with empty baseDir should return error")
}

func TestStore_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	user := auth.NewUser("testuser", "hash", auth.RoleViewer)
	err = store.Create(ctx, user)
	require.NoError(t, err, "Create() failed")

	// Verify file exists
	filePath := filepath.Join(tmpDir, user.ID+".json")
	_, err = os.Stat(filePath)
	assert.False(t, os.IsNotExist(err), "User file should exist after Create()")

	// Verify file is deleted after Delete
	err = store.Delete(ctx, user.ID)
	require.NoError(t, err, "Delete() failed")
	_, err = os.Stat(filePath)
	assert.True(t, os.IsNotExist(err), "User file should not exist after Delete()")
}
