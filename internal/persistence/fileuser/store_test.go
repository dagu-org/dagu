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

func TestStore_OIDCIdentity(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	// Create OIDC user
	user := auth.NewUser("oidcuser", "hash", auth.RoleViewer)
	user.OIDCIssuer = "https://issuer.example.com"
	user.OIDCSubject = "subject123"
	user.AuthProvider = "oidc"
	err = store.Create(ctx, user)
	require.NoError(t, err, "Create() OIDC user failed")

	t.Run("GetByOIDCIdentity_Found", func(t *testing.T) {
		retrieved, err := store.GetByOIDCIdentity(ctx, "https://issuer.example.com", "subject123")
		require.NoError(t, err, "GetByOIDCIdentity() failed")
		assert.Equal(t, user.ID, retrieved.ID)
		assert.Equal(t, user.Username, retrieved.Username)
	})

	t.Run("GetByOIDCIdentity_NotFound", func(t *testing.T) {
		_, err := store.GetByOIDCIdentity(ctx, "https://other-issuer.com", "other-subject")
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityNotFound)
	})

	t.Run("GetByOIDCIdentity_EmptyIssuer", func(t *testing.T) {
		_, err := store.GetByOIDCIdentity(ctx, "", "subject123")
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityNotFound)
	})

	t.Run("GetByOIDCIdentity_EmptySubject", func(t *testing.T) {
		_, err := store.GetByOIDCIdentity(ctx, "https://issuer.example.com", "")
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityNotFound)
	})
}

func TestStore_Update_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	t.Run("Update_NilUser", func(t *testing.T) {
		err := store.Update(ctx, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "nil")
	})

	t.Run("Update_EmptyID", func(t *testing.T) {
		user := &auth.User{Username: "test"}
		err := store.Update(ctx, user)
		assert.ErrorIs(t, err, auth.ErrInvalidUserID)
	})

	t.Run("Update_EmptyUsername", func(t *testing.T) {
		user := &auth.User{ID: "some-id", Username: ""}
		err := store.Update(ctx, user)
		assert.ErrorIs(t, err, auth.ErrInvalidUsername)
	})

	t.Run("Update_UsernameChange_Success", func(t *testing.T) {
		// Create user
		user := auth.NewUser("original", "hash", auth.RoleViewer)
		err := store.Create(ctx, user)
		require.NoError(t, err)

		// Update username
		user.Username = "newname"
		err = store.Update(ctx, user)
		require.NoError(t, err)

		// Verify new username works
		retrieved, err := store.GetByUsername(ctx, "newname")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)

		// Verify old username doesn't work
		_, err = store.GetByUsername(ctx, "original")
		assert.ErrorIs(t, err, auth.ErrUserNotFound)
	})

	t.Run("Update_UsernameChange_Conflict", func(t *testing.T) {
		// Create two users
		user1 := auth.NewUser("user1", "hash", auth.RoleViewer)
		err := store.Create(ctx, user1)
		require.NoError(t, err)

		user2 := auth.NewUser("user2", "hash", auth.RoleViewer)
		err = store.Create(ctx, user2)
		require.NoError(t, err)

		// Try to change user2's username to user1's username
		user2.Username = "user1"
		err = store.Update(ctx, user2)
		assert.ErrorIs(t, err, auth.ErrUserAlreadyExists)
	})

	t.Run("Update_OIDCIdentity", func(t *testing.T) {
		// Create OIDC user
		user := auth.NewUser("oidcuser2", "hash", auth.RoleViewer)
		user.OIDCIssuer = "https://issuer1.com"
		user.OIDCSubject = "sub1"
		err := store.Create(ctx, user)
		require.NoError(t, err)

		// Update OIDC identity
		user.OIDCIssuer = "https://issuer2.com"
		user.OIDCSubject = "sub2"
		err = store.Update(ctx, user)
		require.NoError(t, err)

		// Old identity should not find user
		_, err = store.GetByOIDCIdentity(ctx, "https://issuer1.com", "sub1")
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityNotFound)

		// New identity should find user
		retrieved, err := store.GetByOIDCIdentity(ctx, "https://issuer2.com", "sub2")
		require.NoError(t, err)
		assert.Equal(t, user.ID, retrieved.ID)
	})

	t.Run("Update_OIDCIdentity_Conflict", func(t *testing.T) {
		// Create two OIDC users
		user1 := auth.NewUser("oidcA", "hash", auth.RoleViewer)
		user1.OIDCIssuer = "https://issuer.com"
		user1.OIDCSubject = "subjectA"
		err := store.Create(ctx, user1)
		require.NoError(t, err)

		user2 := auth.NewUser("oidcB", "hash", auth.RoleViewer)
		user2.OIDCIssuer = "https://issuer.com"
		user2.OIDCSubject = "subjectB"
		err = store.Create(ctx, user2)
		require.NoError(t, err)

		// Try to change user2's OIDC identity to user1's
		user2.OIDCSubject = "subjectA"
		err = store.Update(ctx, user2)
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityAlreadyExists)
	})
}

func TestStore_Delete_OIDCUser(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	t.Run("Delete_RemovesOIDCIndex", func(t *testing.T) {
		// Create OIDC user
		user := auth.NewUser("oidcdelete", "hash", auth.RoleViewer)
		user.OIDCIssuer = "https://issuer.example.com"
		user.OIDCSubject = "deletesubject"
		err := store.Create(ctx, user)
		require.NoError(t, err)

		// Verify OIDC identity works
		_, err = store.GetByOIDCIdentity(ctx, "https://issuer.example.com", "deletesubject")
		require.NoError(t, err)

		// Delete user
		err = store.Delete(ctx, user.ID)
		require.NoError(t, err)

		// Verify OIDC identity no longer works
		_, err = store.GetByOIDCIdentity(ctx, "https://issuer.example.com", "deletesubject")
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityNotFound)
	})

	t.Run("Delete_EmptyID", func(t *testing.T) {
		err := store.Delete(ctx, "")
		assert.ErrorIs(t, err, auth.ErrInvalidUserID)
	})
}

func TestStore_Create_EdgeCases(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	store, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()

	t.Run("Create_NilUser", func(t *testing.T) {
		err := store.Create(ctx, nil)
		assert.Error(t, err)
	})

	t.Run("Create_EmptyID", func(t *testing.T) {
		user := &auth.User{Username: "test"}
		err := store.Create(ctx, user)
		assert.ErrorIs(t, err, auth.ErrInvalidUserID)
	})

	t.Run("Create_EmptyUsername", func(t *testing.T) {
		user := &auth.User{ID: "some-id", Username: ""}
		err := store.Create(ctx, user)
		assert.ErrorIs(t, err, auth.ErrInvalidUsername)
	})

	t.Run("Create_DuplicateOIDCIdentity", func(t *testing.T) {
		// Create first OIDC user
		user1 := auth.NewUser("oidc1", "hash", auth.RoleViewer)
		user1.OIDCIssuer = "https://issuer.com"
		user1.OIDCSubject = "same-subject"
		err := store.Create(ctx, user1)
		require.NoError(t, err)

		// Try to create second user with same OIDC identity
		user2 := auth.NewUser("oidc2", "hash", auth.RoleViewer)
		user2.OIDCIssuer = "https://issuer.com"
		user2.OIDCSubject = "same-subject"
		err = store.Create(ctx, user2)
		assert.ErrorIs(t, err, auth.ErrOIDCIdentityAlreadyExists)
	})
}

func TestStore_RebuildIndex_OIDCUsers(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	require.NoError(t, err, "failed to create temp dir")
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Create store and add OIDC user
	store1, err := New(tmpDir)
	require.NoError(t, err, "failed to create store")

	ctx := context.Background()
	user := auth.NewUser("oidcrebuild", "hash", auth.RoleAdmin)
	user.OIDCIssuer = "https://rebuild-issuer.com"
	user.OIDCSubject = "rebuild-subject"
	err = store1.Create(ctx, user)
	require.NoError(t, err, "Create() failed")

	// Create new store instance (simulates restart)
	store2, err := New(tmpDir)
	require.NoError(t, err, "failed to create second store")

	// Verify OIDC identity is indexed after rebuild
	retrieved, err := store2.GetByOIDCIdentity(ctx, "https://rebuild-issuer.com", "rebuild-subject")
	require.NoError(t, err, "GetByOIDCIdentity() after rebuild failed")
	assert.Equal(t, user.ID, retrieved.ID)
}
