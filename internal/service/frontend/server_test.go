// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authmodel "github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/persis/fileuser"
)

// testConfig creates a minimal config for initBuiltinAuthService tests.
// All directories point to subdirectories of the given temp dir.
func testConfig(tmpDir string, ia config.InitialAdmin) *config.Config {
	return &config.Config{
		Paths: config.PathsConfig{
			UsersDir:    filepath.Join(tmpDir, "users"),
			APIKeysDir:  filepath.Join(tmpDir, "apikeys"),
			WebhooksDir: filepath.Join(tmpDir, "webhooks"),
			DataDir:     filepath.Join(tmpDir, "data"),
		},
		Server: config.Server{
			Auth: config.Auth{
				Mode: config.AuthModeBuiltin,
				Builtin: config.AuthBuiltin{
					Token: config.TokenConfig{
						Secret: "test-secret-for-jwt-signing",
						TTL:    24 * time.Hour,
					},
					InitialAdmin: ia,
				},
			},
		},
	}
}

func TestInitBuiltinAuthService_AutoProvision(t *testing.T) {
	t.Parallel()

	t.Run("ProvisionesAdminWhenNoUsers", func(t *testing.T) {
		t.Parallel()
		cfg := testConfig(t.TempDir(), config.InitialAdmin{
			Username: "testadmin",
			Password: "securepass123",
		})

		result, setupRequired, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.NoError(t, err)
		assert.False(t, setupRequired, "setup should not be required after auto-provisioning")

		// Verify user was created
		count, err := result.AuthService.CountUsers(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)

		// Verify user has correct role and username
		user, err := result.UserStore.GetByUsername(context.Background(), "testadmin")
		require.NoError(t, err)
		assert.Equal(t, "testadmin", user.Username)
		assert.Equal(t, authmodel.RoleAdmin, user.Role)
	})

	t.Run("SkipsWhenUsersExist", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := testConfig(tmpDir, config.InitialAdmin{
			Username: "testadmin",
			Password: "securepass123",
		})

		// Pre-create a user directly in the store
		store, err := fileuser.New(cfg.Paths.UsersDir)
		require.NoError(t, err)
		existing := authmodel.NewUser("existinguser", "$2a$12$dummyhash000000000000000000000000000000000000000000", authmodel.RoleAdmin)
		require.NoError(t, store.Create(context.Background(), existing))

		result, setupRequired, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.NoError(t, err)
		assert.False(t, setupRequired)

		// Verify no additional user was created
		count, err := result.AuthService.CountUsers(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})

	t.Run("SkipsWhenNotConfigured", func(t *testing.T) {
		t.Parallel()
		cfg := testConfig(t.TempDir(), config.InitialAdmin{})

		_, setupRequired, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.NoError(t, err)
		assert.True(t, setupRequired, "setup should be required when initial_admin is not configured")
	})

	t.Run("FailsOnInvalidPassword", func(t *testing.T) {
		t.Parallel()
		cfg := testConfig(t.TempDir(), config.InitialAdmin{
			Username: "testadmin",
			Password: "short", // less than 8 characters
		})

		_, _, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to auto-provision initial admin user")
	})

	t.Run("Idempotent", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		cfg := testConfig(tmpDir, config.InitialAdmin{
			Username: "testadmin",
			Password: "securepass123",
		})

		// First call: provisions the user
		_, setupRequired, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.NoError(t, err)
		assert.False(t, setupRequired)

		// Second call: should not create a duplicate
		result, setupRequired, err := initBuiltinAuthService(context.Background(), cfg, nil)
		require.NoError(t, err)
		assert.False(t, setupRequired)

		count, err := result.AuthService.CountUsers(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}

// TestInitBuiltinAuthService_UserCanAuthenticate verifies that the auto-provisioned
// user can actually authenticate (password was hashed correctly).
func TestInitBuiltinAuthService_UserCanAuthenticate(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t.TempDir(), config.InitialAdmin{
		Username: "authadmin",
		Password: "mypassword123",
	})

	result, _, err := initBuiltinAuthService(context.Background(), cfg, nil)
	require.NoError(t, err)

	// Authenticate via the auth service
	user, err := result.AuthService.Authenticate(context.Background(), "authadmin", "mypassword123")
	require.NoError(t, err)
	assert.Equal(t, "authadmin", user.Username)
	assert.Equal(t, authmodel.RoleAdmin, user.Role)

	// Wrong password should fail
	_, err = result.AuthService.Authenticate(context.Background(), "authadmin", "wrongpassword")
	require.Error(t, err)
}
