// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package auth

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/persistence/fileapikey"
	"github.com/dagu-org/dagu/internal/persistence/fileuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestService(t *testing.T) (*Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "auth-service-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	store, err := fileuser.New(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		t.Fatalf("failed to create store: %v", err)
	}

	config := Config{
		TokenSecret: "test-secret-key-for-jwt-signing",
		TokenTTL:    time.Hour,
		BcryptCost:  4, // Low cost for faster tests
	}

	svc := New(store, config)
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return svc, cleanup
}

func TestService_CreateUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	if user.Username != "testuser" {
		t.Errorf("CreateUser() username = %v, want %v", user.Username, "testuser")
	}
	if user.Role != auth.RoleManager {
		t.Errorf("CreateUser() role = %v, want %v", user.Role, auth.RoleManager)
	}
	if user.PasswordHash == "" {
		t.Error("CreateUser() password hash should not be empty")
	}
	if user.PasswordHash == "password123" {
		t.Error("CreateUser() password should be hashed")
	}
}

func TestService_CreateUser_WeakPassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "short", // Too short
		Role:     auth.RoleViewer,
	})
	if err == nil {
		t.Error("CreateUser() with weak password should return error")
	}
}

func TestService_Authenticate(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	_, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Test successful authentication
	user, err := svc.Authenticate(ctx, "testuser", "password123")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("Authenticate() username = %v, want %v", user.Username, "testuser")
	}

	// Test wrong password
	_, err = svc.Authenticate(ctx, "testuser", "wrongpassword")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() with wrong password error = %v, want %v", err, ErrInvalidCredentials)
	}

	// Test non-existent user
	_, err = svc.Authenticate(ctx, "nonexistent", "password123")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() with non-existent user error = %v, want %v", err, ErrInvalidCredentials)
	}
}

func TestService_GenerateAndValidateToken(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Generate token
	tokenResult, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}
	if tokenResult.Token == "" {
		t.Error("GenerateToken() returned empty token")
	}
	if tokenResult.ExpiresAt.IsZero() {
		t.Error("GenerateToken() returned zero expiry time")
	}

	// Validate token
	claims, err := svc.ValidateToken(tokenResult.Token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}
	if claims.UserID != user.ID {
		t.Errorf("ValidateToken() userID = %v, want %v", claims.UserID, user.ID)
	}
	if claims.Username != user.Username {
		t.Errorf("ValidateToken() username = %v, want %v", claims.Username, user.Username)
	}
	if claims.Role != user.Role {
		t.Errorf("ValidateToken() role = %v, want %v", claims.Role, user.Role)
	}
}

func TestService_ValidateToken_Invalid(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	// Test invalid token
	_, err := svc.ValidateToken("invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("ValidateToken() with invalid token error = %v, want %v", err, ErrInvalidToken)
	}
}

func TestService_GetUserFromToken(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Generate token
	tokenResult, err := svc.GenerateToken(user)
	if err != nil {
		t.Fatalf("GenerateToken() error = %v", err)
	}

	// Get user from token
	retrieved, err := svc.GetUserFromToken(ctx, tokenResult.Token)
	if err != nil {
		t.Fatalf("GetUserFromToken() error = %v", err)
	}
	if retrieved.ID != user.ID {
		t.Errorf("GetUserFromToken() ID = %v, want %v", retrieved.ID, user.ID)
	}
}

func TestService_ChangePassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "oldpassword1",
		Role:     auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Change password
	err = svc.ChangePassword(ctx, user.ID, "oldpassword1", "newpassword1")
	if err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}

	// Verify old password no longer works
	_, err = svc.Authenticate(ctx, "testuser", "oldpassword1")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() with old password should fail")
	}

	// Verify new password works
	_, err = svc.Authenticate(ctx, "testuser", "newpassword1")
	if err != nil {
		t.Errorf("Authenticate() with new password error = %v", err)
	}
}

func TestService_ChangePassword_WrongOldPassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Try to change with wrong old password
	err = svc.ChangePassword(ctx, user.ID, "wrongpassword", "newpassword1")
	if err != ErrPasswordMismatch {
		t.Errorf("ChangePassword() with wrong old password error = %v, want %v", err, ErrPasswordMismatch)
	}
}

func TestService_EnsureAdminUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// First call should create admin
	password, created, err := svc.EnsureAdminUser(ctx, "admin", "adminpass1")
	if err != nil {
		t.Fatalf("EnsureAdminUser() error = %v", err)
	}
	if !created {
		t.Error("EnsureAdminUser() should return created=true")
	}
	if password != "adminpass1" {
		t.Errorf("EnsureAdminUser() password = %v, want %v", password, "adminpass1")
	}

	// Verify admin can authenticate
	_, err = svc.Authenticate(ctx, "admin", "adminpass1")
	if err != nil {
		t.Errorf("Authenticate() admin error = %v", err)
	}

	// Second call should not create
	_, created, err = svc.EnsureAdminUser(ctx, "admin2", "adminpass2")
	if err != nil {
		t.Fatalf("EnsureAdminUser() second call error = %v", err)
	}
	if created {
		t.Error("EnsureAdminUser() should return created=false when users exist")
	}
}

func TestService_EnsureAdminUser_GeneratePassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Call with empty password should generate one
	password, created, err := svc.EnsureAdminUser(ctx, "admin", "")
	if err != nil {
		t.Fatalf("EnsureAdminUser() error = %v", err)
	}
	if !created {
		t.Error("EnsureAdminUser() should return created=true")
	}
	if password == "" {
		t.Error("EnsureAdminUser() should generate a password")
	}
	if len(password) < 8 {
		t.Error("Generated password should be at least 8 characters")
	}

	// Verify admin can authenticate with generated password
	_, err = svc.Authenticate(ctx, "admin", password)
	if err != nil {
		t.Errorf("Authenticate() admin with generated password error = %v", err)
	}
}

func TestService_DeleteUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Delete user
	err = svc.DeleteUser(ctx, user.ID, "other-user-id")
	if err != nil {
		t.Fatalf("DeleteUser() error = %v", err)
	}

	// Verify user is deleted
	_, err = svc.GetUser(ctx, user.ID)
	if err != auth.ErrUserNotFound {
		t.Errorf("GetUser() after delete error = %v, want %v", err, auth.ErrUserNotFound)
	}
}

func TestService_DeleteUser_CannotDeleteSelf(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Try to delete self
	err = svc.DeleteUser(ctx, user.ID, user.ID)
	if err != ErrCannotDeleteSelf {
		t.Errorf("DeleteUser() self error = %v, want %v", err, ErrCannotDeleteSelf)
	}
}

func TestService_UpdateUser(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Update role
	newRole := auth.RoleAdmin
	updated, err := svc.UpdateUser(ctx, user.ID, UpdateUserInput{
		Role: &newRole,
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Role != auth.RoleAdmin {
		t.Errorf("UpdateUser() role = %v, want %v", updated.Role, auth.RoleAdmin)
	}

	// Update username
	newUsername := "newusername"
	updated, err = svc.UpdateUser(ctx, user.ID, UpdateUserInput{
		Username: &newUsername,
	})
	if err != nil {
		t.Fatalf("UpdateUser() error = %v", err)
	}
	if updated.Username != "newusername" {
		t.Errorf("UpdateUser() username = %v, want %v", updated.Username, "newusername")
	}
}

func TestService_ListUsers(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple users
	for i := 0; i < 3; i++ {
		_, err := svc.CreateUser(ctx, CreateUserInput{
			Username: fmt.Sprintf("user%d", i),
			Password: "password123",
			Role:     auth.RoleViewer,
		})
		if err != nil {
			t.Fatalf("CreateUser() error = %v", err)
		}
	}

	// List users
	users, err := svc.ListUsers(ctx)
	if err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if len(users) != 3 {
		t.Errorf("ListUsers() returned %d users, want 3", len(users))
	}
}

func TestService_ResetPassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "oldpassword1",
		Role:     auth.RoleManager,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Reset password (admin action, doesn't require old password)
	err = svc.ResetPassword(ctx, user.ID, "newpassword1")
	if err != nil {
		t.Fatalf("ResetPassword() error = %v", err)
	}

	// Verify old password no longer works
	_, err = svc.Authenticate(ctx, "testuser", "oldpassword1")
	if err != ErrInvalidCredentials {
		t.Errorf("Authenticate() with old password should fail")
	}

	// Verify new password works
	_, err = svc.Authenticate(ctx, "testuser", "newpassword1")
	if err != nil {
		t.Errorf("Authenticate() with new password error = %v", err)
	}
}

func TestService_ResetPassword_WeakPassword(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user, err := svc.CreateUser(ctx, CreateUserInput{
		Username: "testuser",
		Password: "password123",
		Role:     auth.RoleViewer,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}

	// Try to reset with weak password
	err = svc.ResetPassword(ctx, user.ID, "weak")
	if err == nil {
		t.Error("ResetPassword() with weak password should return error")
	}
}

// ============================================================================
// API Key Tests
// ============================================================================

func setupTestServiceWithAPIKeys(t *testing.T) (*Service, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "auth-service-test-*")
	require.NoError(t, err, "failed to create temp dir")

	userStore, err := fileuser.New(tmpDir)
	require.NoError(t, err, "failed to create user store")

	apiKeyDir := filepath.Join(tmpDir, "apikeys")
	apiKeyStore, err := fileapikey.New(apiKeyDir)
	require.NoError(t, err, "failed to create API key store")

	config := Config{
		TokenSecret: "test-secret-key-for-jwt-signing",
		TokenTTL:    time.Hour,
		BcryptCost:  4, // Low cost for faster tests
	}

	svc := New(userStore, config, WithAPIKeyStore(apiKeyStore))
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}

	return svc, cleanup
}

func TestService_CreateAPIKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name:        "test-key",
		Description: "Test API key",
		Role:        auth.RoleManager,
	}, "creator-id")
	require.NoError(t, err)
	require.NotNil(t, result.APIKey)

	assert.Equal(t, "test-key", result.APIKey.Name)
	assert.Equal(t, "Test API key", result.APIKey.Description)
	assert.Equal(t, auth.RoleManager, result.APIKey.Role)
	assert.Equal(t, "creator-id", result.APIKey.CreatedBy)
	assert.NotEmpty(t, result.FullKey)
	assert.True(t, strings.HasPrefix(result.FullKey, "dagu_"), "full key should start with 'dagu_'")
	assert.NotEmpty(t, result.APIKey.KeyPrefix)
	assert.NotEmpty(t, result.APIKey.KeyHash)
}

func TestService_CreateAPIKey_EmptyName(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.ErrorIs(t, err, auth.ErrInvalidAPIKeyName)
}

func TestService_CreateAPIKey_InvalidRole(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.Role("invalid"),
	}, "creator-id")
	require.Error(t, err, "CreateAPIKey() with invalid role should return error")
}

func TestService_CreateAPIKey_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_GetAPIKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name:        "test-key",
		Description: "Test API key",
		Role:        auth.RoleManager,
	}, "creator-id")
	require.NoError(t, err)

	// Get the API key
	apiKey, err := svc.GetAPIKey(ctx, result.APIKey.ID)
	require.NoError(t, err)

	assert.Equal(t, result.APIKey.ID, apiKey.ID)
	assert.Equal(t, "test-key", apiKey.Name)
}

func TestService_GetAPIKey_NotFound(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetAPIKey(ctx, "non-existent-id")
	require.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestService_GetAPIKey_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.GetAPIKey(ctx, "some-id")
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_ListAPIKeys(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create multiple API keys
	for i := 0; i < 3; i++ {
		_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
			Name: fmt.Sprintf("key-%d", i),
			Role: auth.RoleViewer,
		}, "creator-id")
		require.NoError(t, err)
	}

	// List API keys
	keys, err := svc.ListAPIKeys(ctx)
	require.NoError(t, err)
	assert.Len(t, keys, 3)
}

func TestService_ListAPIKeys_Empty(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	keys, err := svc.ListAPIKeys(ctx)
	require.NoError(t, err)
	assert.Empty(t, keys)
}

func TestService_ListAPIKeys_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.ListAPIKeys(ctx)
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_UpdateAPIKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name:        "original-name",
		Description: "Original description",
		Role:        auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Update the API key
	newName := "updated-name"
	newDesc := "Updated description"
	newRole := auth.RoleAdmin
	updated, err := svc.UpdateAPIKey(ctx, result.APIKey.ID, UpdateAPIKeyInput{
		Name:        &newName,
		Description: &newDesc,
		Role:        &newRole,
	})
	require.NoError(t, err)

	assert.Equal(t, "updated-name", updated.Name)
	assert.Equal(t, "Updated description", updated.Description)
	assert.Equal(t, auth.RoleAdmin, updated.Role)
}

func TestService_UpdateAPIKey_PartialUpdate(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name:        "original-name",
		Description: "Original description",
		Role:        auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Update only the name
	newName := "updated-name"
	updated, err := svc.UpdateAPIKey(ctx, result.APIKey.ID, UpdateAPIKeyInput{
		Name: &newName,
	})
	require.NoError(t, err)

	assert.Equal(t, "updated-name", updated.Name)
	// Other fields should remain unchanged
	assert.Equal(t, "Original description", updated.Description)
	assert.Equal(t, auth.RoleViewer, updated.Role)
}

func TestService_UpdateAPIKey_InvalidRole(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Try to update with invalid role
	invalidRole := auth.Role("invalid")
	_, err = svc.UpdateAPIKey(ctx, result.APIKey.ID, UpdateAPIKeyInput{
		Role: &invalidRole,
	})
	require.Error(t, err, "UpdateAPIKey() with invalid role should return error")
}

func TestService_UpdateAPIKey_NotFound(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	newName := "updated-name"
	_, err := svc.UpdateAPIKey(ctx, "non-existent-id", UpdateAPIKeyInput{
		Name: &newName,
	})
	require.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestService_UpdateAPIKey_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	newName := "updated-name"
	_, err := svc.UpdateAPIKey(ctx, "some-id", UpdateAPIKeyInput{
		Name: &newName,
	})
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_DeleteAPIKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Delete the API key
	err = svc.DeleteAPIKey(ctx, result.APIKey.ID)
	require.NoError(t, err)

	// Verify it's deleted
	_, err = svc.GetAPIKey(ctx, result.APIKey.ID)
	require.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestService_DeleteAPIKey_NotFound(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.DeleteAPIKey(ctx, "non-existent-id")
	require.ErrorIs(t, err, auth.ErrAPIKeyNotFound)
}

func TestService_DeleteAPIKey_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	err := svc.DeleteAPIKey(ctx, "some-id")
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_ValidateAPIKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleManager,
	}, "creator-id")
	require.NoError(t, err)

	// Validate the API key
	apiKey, err := svc.ValidateAPIKey(ctx, result.FullKey)
	require.NoError(t, err)

	assert.Equal(t, result.APIKey.ID, apiKey.ID)
	assert.Equal(t, "test-key", apiKey.Name)
	assert.Equal(t, auth.RoleManager, apiKey.Role)
}

func TestService_ValidateAPIKey_InvalidPrefix(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.ValidateAPIKey(ctx, "invalid_prefix_key")
	require.ErrorIs(t, err, ErrInvalidAPIKey)
}

func TestService_ValidateAPIKey_WrongKey(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Try to validate with wrong key (correct prefix but wrong value)
	_, err = svc.ValidateAPIKey(ctx, "dagu_wrongkeywrongkeywrongkeywrongkey")
	require.ErrorIs(t, err, ErrInvalidAPIKey)
}

func TestService_ValidateAPIKey_NotConfigured(t *testing.T) {
	svc, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.ValidateAPIKey(ctx, "dagu_somekey")
	require.ErrorIs(t, err, ErrAPIKeyNotConfigured)
}

func TestService_HasAPIKeyStore(t *testing.T) {
	// Test with API key store
	svcWithStore, cleanup1 := setupTestServiceWithAPIKeys(t)
	defer cleanup1()

	assert.True(t, svcWithStore.HasAPIKeyStore(), "HasAPIKeyStore() should return true when configured")

	// Test without API key store
	svcWithoutStore, cleanup2 := setupTestService(t)
	defer cleanup2()

	assert.False(t, svcWithoutStore.HasAPIKeyStore(), "HasAPIKeyStore() should return false when not configured")
}

func TestService_CreateAPIKey_EmptyCreatorID(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	_, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "test-key",
		Role: auth.RoleViewer,
	}, "") // Empty creator ID
	require.ErrorIs(t, err, ErrInvalidCreatorID)
}

func TestService_ValidateAPIKey_UpdatesLastUsed(t *testing.T) {
	svc, cleanup := setupTestServiceWithAPIKeys(t)
	defer cleanup()

	ctx := context.Background()

	// Create an API key
	result, err := svc.CreateAPIKey(ctx, CreateAPIKeyInput{
		Name: "lastused-key",
		Role: auth.RoleViewer,
	}, "creator-id")
	require.NoError(t, err)

	// Verify LastUsedAt is nil initially
	apiKey, err := svc.GetAPIKey(ctx, result.APIKey.ID)
	require.NoError(t, err)
	assert.Nil(t, apiKey.LastUsedAt, "LastUsedAt should be nil initially")

	// Validate the API key (this should update LastUsedAt asynchronously)
	_, err = svc.ValidateAPIKey(ctx, result.FullKey)
	require.NoError(t, err)

	// Wait for async update to complete
	time.Sleep(100 * time.Millisecond)

	// Verify LastUsedAt is now populated
	apiKey2, err := svc.GetAPIKey(ctx, result.APIKey.ID)
	require.NoError(t, err)
	require.NotNil(t, apiKey2.LastUsedAt, "LastUsedAt should be populated after validation")
}

func TestGenerateAPIKey(t *testing.T) {
	// Test API key generation
	keyParts, err := generateAPIKey(4) // Low cost for fast tests
	require.NoError(t, err)

	// Verify full key has correct prefix
	assert.True(t, strings.HasPrefix(keyParts.fullKey, "dagu_"), "Full key should start with 'dagu_'")

	// Verify key prefix is correct length
	assert.Len(t, keyParts.keyPrefix, apiKeyPrefixLength, "Key prefix should be %d characters", apiKeyPrefixLength)

	// Verify key prefix matches start of full key
	assert.Equal(t, keyParts.fullKey[:apiKeyPrefixLength], keyParts.keyPrefix, "Key prefix should match start of full key")

	// Verify hash is valid bcrypt hash
	assert.NotEmpty(t, keyParts.keyHash, "Key hash should not be empty")
	assert.True(t, strings.HasPrefix(keyParts.keyHash, "$2"), "Key hash should be bcrypt format")

	// Verify full key is long enough (should be at least 40 chars: 5 prefix + 32 bytes base58)
	assert.GreaterOrEqual(t, len(keyParts.fullKey), 40, "Full key should be at least 40 characters")
}

func TestGenerateAPIKey_UniqueKeys(t *testing.T) {
	// Generate multiple keys and verify they are unique
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		keyParts, err := generateAPIKey(4)
		require.NoError(t, err)
		assert.False(t, keys[keyParts.fullKey], "Generated key should be unique")
		keys[keyParts.fullKey] = true
	}
}
