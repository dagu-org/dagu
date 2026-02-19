package fileauth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/persis/fileauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultCacheLimits() config.CacheLimits {
	return config.CacheModeNormal.Limits()
}

func setupStore(t *testing.T) *fileauth.Store {
	t.Helper()
	tmpDir := t.TempDir()
	store, err := fileauth.New(context.Background(), fileauth.Config{
		UsersDir:    filepath.Join(tmpDir, "users"),
		APIKeysDir:  filepath.Join(tmpDir, "apikeys"),
		WebhooksDir: filepath.Join(tmpDir, "webhooks"),
		CacheLimits: defaultCacheLimits(),
	})
	require.NoError(t, err)
	return store
}

func TestNew_Success(t *testing.T) {
	store := setupStore(t)
	assert.NotNil(t, store.Users())
	assert.NotNil(t, store.APIKeys())
	assert.NotNil(t, store.Webhooks())
}

func TestNew_InvalidDir(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a file where a directory is expected — sub-store creation should fail.
	blocker := filepath.Join(tmpDir, "blocker")
	require.NoError(t, os.WriteFile(blocker, []byte("not-a-dir"), 0600))

	_, err := fileauth.New(context.Background(), fileauth.Config{
		UsersDir:    filepath.Join(blocker, "users"),
		APIKeysDir:  filepath.Join(tmpDir, "apikeys"),
		WebhooksDir: filepath.Join(tmpDir, "webhooks"),
		CacheLimits: defaultCacheLimits(),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "fileauth:")
}

func TestStore_SubStoreWiring(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()

	// Verify user sub-store works end-to-end
	user := auth.NewUser("admin", "hashedpw", auth.RoleAdmin)
	require.NoError(t, store.Users().Create(ctx, user))
	got, err := store.Users().GetByUsername(ctx, "admin")
	require.NoError(t, err)
	assert.Equal(t, "admin", got.Username)

	// Verify API key sub-store works end-to-end
	apiKey, err := auth.NewAPIKey("test-key", "desc", auth.RoleAdmin, "hash123", "prefix", user.ID)
	require.NoError(t, err)
	require.NoError(t, store.APIKeys().Create(ctx, apiKey))
	gotKey, err := store.APIKeys().GetByID(ctx, apiKey.ID)
	require.NoError(t, err)
	assert.Equal(t, "test-key", gotKey.Name)

	// Verify webhook sub-store works end-to-end
	webhook, err := auth.NewWebhook("my-dag", "tokenhash", "prefix", user.ID)
	require.NoError(t, err)
	require.NoError(t, store.Webhooks().Create(ctx, webhook))
	gotWebhook, err := store.Webhooks().GetByDAGName(ctx, "my-dag")
	require.NoError(t, err)
	assert.Equal(t, "my-dag", gotWebhook.DAGName)
}
