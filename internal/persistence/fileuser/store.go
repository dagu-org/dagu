// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package fileuser provides a file-based implementation of the UserStore interface.
package fileuser

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/auth"
)

const (
	// userFileExtension is the file extension for user files.
	userFileExtension = ".json"
	// userDirPermissions is the permission mode for the users directory.
	userDirPermissions = 0750
	// userFilePermissions is the permission mode for user files.
	userFilePermissions = 0600
)

// Store implements auth.UserStore using the local filesystem.
// Users are stored as individual JSON files in the configured directory.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	// mu protects the index maps
	mu sync.RWMutex
	// byID maps user ID to file path
	byID map[string]string
	// byUsername maps username to user ID
	byUsername map[string]string
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// New creates a new file-based user store.
// New creates a file-backed Store that persists users as per-user JSON files in baseDir.
// The baseDir must be non-empty; provided Option functions are applied to the store.
// If baseDir does not exist it is created with directory permissions 0750, and an initial
// in-memory index is built from existing user files. Returns an error on invalid input,
// failure to create the directory, or failure to build the initial index.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileuser: baseDir cannot be empty")
	}

	store := &Store{
		baseDir:    baseDir,
		byID:       make(map[string]string),
		byUsername: make(map[string]string),
	}

	for _, opt := range opts {
		opt(store)
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, userDirPermissions); err != nil {
		return nil, fmt.Errorf("fileuser: failed to create directory %s: %w", baseDir, err)
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("fileuser: failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the directory and rebuilds the in-memory index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing index
	s.byID = make(map[string]string)
	s.byUsername = make(map[string]string)

	// Scan directory for user files
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != userFileExtension {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		user, err := s.loadUserFromFile(filePath)
		if err != nil {
			// Log warning but continue - don't fail entire index for one bad file
			slog.Warn("Failed to load user file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[user.ID] = filePath
		s.byUsername[user.Username] = user.ID
	}

	return nil
}

// loadUserFromFile reads and parses a user from a JSON file.
func (s *Store) loadUserFromFile(filePath string) (*auth.User, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally from baseDir + userID
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var stored auth.UserForStorage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse user file %s: %w", filePath, err)
	}

	return stored.ToUser(), nil
}

// userFilePath returns the file path for a user ID.
func (s *Store) userFilePath(userID string) string {
	return filepath.Join(s.baseDir, userID+userFileExtension)
}

// Create stores a new user.
func (s *Store) Create(_ context.Context, user *auth.User) error {
	if user == nil {
		return errors.New("fileuser: user cannot be nil")
	}
	if user.ID == "" {
		return auth.ErrInvalidUserID
	}
	if user.Username == "" {
		return auth.ErrInvalidUsername
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if username already exists
	if _, exists := s.byUsername[user.Username]; exists {
		return auth.ErrUserAlreadyExists
	}

	// Check if ID already exists (shouldn't happen with UUIDs, but be safe)
	if _, exists := s.byID[user.ID]; exists {
		return auth.ErrUserAlreadyExists
	}

	// Write user to file
	filePath := s.userFilePath(user.ID)
	if err := s.writeUserToFile(filePath, user); err != nil {
		return err
	}

	// Update index
	s.byID[user.ID] = filePath
	s.byUsername[user.Username] = user.ID

	return nil
}

// writeUserToFile writes a user to a JSON file atomically.
func (s *Store) writeUserToFile(filePath string, user *auth.User) error {
	data, err := json.MarshalIndent(user.ToStorage(), "", "  ")
	if err != nil {
		return fmt.Errorf("fileuser: failed to marshal user: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, userFilePermissions); err != nil {
		return fmt.Errorf("fileuser: failed to write file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempPath)
		return fmt.Errorf("fileuser: failed to rename file %s: %w", filePath, err)
	}

	return nil
}

// GetByID retrieves a user by their unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*auth.User, error) {
	if id == "" {
		return nil, auth.ErrInvalidUserID
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, auth.ErrUserNotFound
	}

	user, err := s.loadUserFromFile(filePath)
	if err != nil {
		// File might have been deleted externally
		if errors.Is(err, os.ErrNotExist) {
			return nil, auth.ErrUserNotFound
		}
		return nil, fmt.Errorf("fileuser: failed to load user %s: %w", id, err)
	}

	return user, nil
}

// GetByUsername retrieves a user by their username.
func (s *Store) GetByUsername(ctx context.Context, username string) (*auth.User, error) {
	if username == "" {
		return nil, auth.ErrInvalidUsername
	}

	s.mu.RLock()
	userID, exists := s.byUsername[username]
	s.mu.RUnlock()

	if !exists {
		return nil, auth.ErrUserNotFound
	}

	return s.GetByID(ctx, userID)
}

// List returns all users in the store.
func (s *Store) List(ctx context.Context) ([]*auth.User, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	users := make([]*auth.User, 0, len(ids))
	for _, id := range ids {
		user, err := s.GetByID(ctx, id)
		if err != nil {
			// Skip users that can't be loaded
			if errors.Is(err, auth.ErrUserNotFound) {
				continue
			}
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

// Update modifies an existing user.
func (s *Store) Update(_ context.Context, user *auth.User) error {
	if user == nil {
		return errors.New("fileuser: user cannot be nil")
	}
	if user.ID == "" {
		return auth.ErrInvalidUserID
	}
	if user.Username == "" {
		return auth.ErrInvalidUsername
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[user.ID]
	if !exists {
		return auth.ErrUserNotFound
	}

	// Load existing user to check for username change
	existingUser, err := s.loadUserFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileuser: failed to load existing user: %w", err)
	}

	// If username changed, check for conflicts and update index
	if existingUser.Username != user.Username {
		if existingID, taken := s.byUsername[user.Username]; taken && existingID != user.ID {
			return auth.ErrUserAlreadyExists
		}
		delete(s.byUsername, existingUser.Username)
		s.byUsername[user.Username] = user.ID
	}

	// Write updated user
	if err := s.writeUserToFile(filePath, user); err != nil {
		// Rollback index change on failure
		if existingUser.Username != user.Username {
			delete(s.byUsername, user.Username)
			s.byUsername[existingUser.Username] = user.ID
		}
		return err
	}

	return nil
}

// Delete removes a user by their ID.
func (s *Store) Delete(_ context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidUserID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return auth.ErrUserNotFound
	}

	// Load user to get username for index cleanup
	user, err := s.loadUserFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileuser: failed to load user for deletion: %w", err)
	}

	// Remove file
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileuser: failed to delete user file: %w", err)
	}

	// Update index
	delete(s.byID, id)
	if user != nil {
		delete(s.byUsername, user.Username)
	} else {
		// File was already gone; remove any username entry that still points to this ID.
		for username, userID := range s.byUsername {
			if userID == id {
				delete(s.byUsername, username)
				break
			}
		}
	}

	return nil
}

// Count returns the total number of users.
func (s *Store) Count(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.byID)), nil
}
