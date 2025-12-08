// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileuser

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/core/auth"
)

func TestStore_CRUD(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store
	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test Create
	user := auth.NewUser("testuser", "hashedpassword", auth.RoleManager)
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test GetByID
	retrieved, err := store.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if retrieved.Username != user.Username {
		t.Errorf("GetByID() username = %v, want %v", retrieved.Username, user.Username)
	}
	if retrieved.Role != user.Role {
		t.Errorf("GetByID() role = %v, want %v", retrieved.Role, user.Role)
	}

	// Test GetByUsername
	retrieved, err = store.GetByUsername(ctx, user.Username)
	if err != nil {
		t.Fatalf("GetByUsername() error = %v", err)
	}
	if retrieved.ID != user.ID {
		t.Errorf("GetByUsername() ID = %v, want %v", retrieved.ID, user.ID)
	}

	// Test List
	users, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(users) != 1 {
		t.Errorf("List() returned %d users, want 1", len(users))
	}

	// Test Count
	count, err := store.Count(ctx)
	if err != nil {
		t.Fatalf("Count() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Count() = %d, want 1", count)
	}

	// Test Update
	user.Role = auth.RoleAdmin
	if err := store.Update(ctx, user); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	retrieved, _ = store.GetByID(ctx, user.ID)
	if retrieved.Role != auth.RoleAdmin {
		t.Errorf("Update() role = %v, want %v", retrieved.Role, auth.RoleAdmin)
	}

	// Test Delete
	if err := store.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	_, err = store.GetByID(ctx, user.ID)
	if err != auth.ErrUserNotFound {
		t.Errorf("GetByID() after delete error = %v, want %v", err, auth.ErrUserNotFound)
	}
}

func TestStore_DuplicateUsername(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Create first user
	user1 := auth.NewUser("testuser", "hash1", auth.RoleViewer)
	if err := store.Create(ctx, user1); err != nil {
		t.Fatalf("Create() first user error = %v", err)
	}

	// Try to create second user with same username
	user2 := auth.NewUser("testuser", "hash2", auth.RoleManager)
	err = store.Create(ctx, user2)
	if err != auth.ErrUserAlreadyExists {
		t.Errorf("Create() duplicate username error = %v, want %v", err, auth.ErrUserAlreadyExists)
	}
}

func TestStore_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()

	// Test GetByID not found
	_, err = store.GetByID(ctx, "nonexistent-id")
	if err != auth.ErrUserNotFound {
		t.Errorf("GetByID() error = %v, want %v", err, auth.ErrUserNotFound)
	}

	// Test GetByUsername not found
	_, err = store.GetByUsername(ctx, "nonexistent-user")
	if err != auth.ErrUserNotFound {
		t.Errorf("GetByUsername() error = %v, want %v", err, auth.ErrUserNotFound)
	}

	// Test Delete not found
	err = store.Delete(ctx, "nonexistent-id")
	if err != auth.ErrUserNotFound {
		t.Errorf("Delete() error = %v, want %v", err, auth.ErrUserNotFound)
	}

	// Test Update not found
	user := auth.NewUser("test", "hash", auth.RoleViewer)
	err = store.Update(ctx, user)
	if err != auth.ErrUserNotFound {
		t.Errorf("Update() error = %v, want %v", err, auth.ErrUserNotFound)
	}
}

func TestStore_RebuildIndex(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create store and add user
	store1, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	user := auth.NewUser("testuser", "hash", auth.RoleAdmin)
	if err := store1.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Create new store instance (simulates restart)
	store2, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create second store: %v", err)
	}

	// Verify user is found after index rebuild
	retrieved, err := store2.GetByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("GetByUsername() after rebuild error = %v", err)
	}
	if retrieved.ID != user.ID {
		t.Errorf("GetByUsername() after rebuild ID = %v, want %v", retrieved.ID, user.ID)
	}
}

func TestStore_EmptyBaseDir(t *testing.T) {
	_, err := New("")
	if err == nil {
		t.Error("New() with empty baseDir should return error")
	}
}

func TestStore_FileExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "fileuser-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	store, err := New(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	ctx := context.Background()
	user := auth.NewUser("testuser", "hash", auth.RoleViewer)
	if err := store.Create(ctx, user); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Verify file exists
	filePath := filepath.Join(tmpDir, user.ID+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("User file should exist after Create()")
	}

	// Verify file is deleted after Delete
	if err := store.Delete(ctx, user.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("User file should not exist after Delete()")
	}
}
