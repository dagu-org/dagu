// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package filewebhook provides a file-based implementation of the WebhookStore interface.
package filewebhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/auth"
	"github.com/dagu-org/dagu/internal/common/fileutil"
)

const (
	// webhookFileExtension is the file extension for webhook files.
	webhookFileExtension = ".json"
	// webhookDirPermissions is the permission mode for the webhooks directory.
	webhookDirPermissions = 0750
	// webhookFilePermissions is the permission mode for webhook files.
	webhookFilePermissions = 0600
)

var _ auth.WebhookStore = (*Store)(nil)

// Store implements auth.WebhookStore using the local filesystem.
// Webhooks are stored as individual JSON files in the configured directory.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	// mu protects the index maps
	mu sync.RWMutex
	// byID maps webhook ID to file path
	byID map[string]string
	// byDAGName maps DAG name to webhook ID (enforces 1:1 relationship)
	byDAGName map[string]string

	// fileCache caches webhook objects to avoid repeated file reads
	fileCache *fileutil.Cache[*auth.Webhook]
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// WithFileCache sets the file cache for webhook objects.
func WithFileCache(cache *fileutil.Cache[*auth.Webhook]) Option {
	return func(s *Store) {
		s.fileCache = cache
	}
}

// New creates a new file-based webhook store.
// The baseDir must be non-empty; provided Option functions are applied to the store.
// If baseDir does not exist it is created with directory permissions 0750, and an initial
// in-memory index is built from existing webhook files.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("filewebhook: baseDir cannot be empty")
	}

	store := &Store{
		baseDir:   baseDir,
		byID:      make(map[string]string),
		byDAGName: make(map[string]string),
	}

	for _, opt := range opts {
		opt(store)
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, webhookDirPermissions); err != nil {
		return nil, fmt.Errorf("filewebhook: failed to create directory %s: %w", baseDir, err)
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("filewebhook: failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the directory and rebuilds the in-memory index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing index
	s.byID = make(map[string]string)
	s.byDAGName = make(map[string]string)

	// Scan directory for webhook files
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != webhookFileExtension {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		webhook, err := s.loadWebhookFromFile(filePath)
		if err != nil {
			// Log warning but continue - don't fail entire index for one bad file
			slog.Warn("Failed to load webhook file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[webhook.ID] = filePath
		s.byDAGName[webhook.DAGName] = webhook.ID
	}

	return nil
}

// loadWebhookFromFile reads and parses a webhook from a JSON file.
func (s *Store) loadWebhookFromFile(filePath string) (*auth.Webhook, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally from baseDir + webhookID
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var stored auth.WebhookForStorage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse webhook file %s: %w", filePath, err)
	}

	return stored.ToWebhook(), nil
}

// webhookFilePath returns the file path for a webhook ID.
func (s *Store) webhookFilePath(webhookID string) string {
	return filepath.Join(s.baseDir, webhookID+webhookFileExtension)
}

// Create stores a new webhook.
func (s *Store) Create(_ context.Context, webhook *auth.Webhook) error {
	if webhook == nil {
		return errors.New("filewebhook: webhook cannot be nil")
	}
	if webhook.ID == "" {
		return auth.ErrInvalidWebhookID
	}
	if webhook.DAGName == "" {
		return auth.ErrInvalidWebhookDAGName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if DAG already has a webhook (enforces 1:1 relationship)
	if _, exists := s.byDAGName[webhook.DAGName]; exists {
		return auth.ErrWebhookAlreadyExists
	}

	// Check if ID already exists (shouldn't happen with UUIDs, but be safe)
	if _, exists := s.byID[webhook.ID]; exists {
		return auth.ErrWebhookAlreadyExists
	}

	// Write webhook to file
	filePath := s.webhookFilePath(webhook.ID)
	if err := s.writeWebhookToFile(filePath, webhook); err != nil {
		return err
	}

	// Update index
	s.byID[webhook.ID] = filePath
	s.byDAGName[webhook.DAGName] = webhook.ID

	return nil
}

// writeWebhookToFile writes a webhook to a JSON file atomically.
func (s *Store) writeWebhookToFile(filePath string, webhook *auth.Webhook) error {
	data, err := json.MarshalIndent(webhook.ToStorage(), "", "  ")
	if err != nil {
		return fmt.Errorf("filewebhook: failed to marshal webhook: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, webhookFilePermissions); err != nil {
		return fmt.Errorf("filewebhook: failed to write file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempPath)
		return fmt.Errorf("filewebhook: failed to rename file %s: %w", filePath, err)
	}

	return nil
}

// GetByID retrieves a webhook by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*auth.Webhook, error) {
	if id == "" {
		return nil, auth.ErrInvalidWebhookID
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	if !exists {
		s.mu.RUnlock()
		return nil, auth.ErrWebhookNotFound
	}

	var webhook *auth.Webhook
	var err error

	// Use cache if available, otherwise load directly
	if s.fileCache != nil {
		webhook, err = s.fileCache.LoadLatest(filePath, func() (*auth.Webhook, error) {
			return s.loadWebhookFromFile(filePath)
		})
	} else {
		// Load file while still holding the read lock to prevent TOCTOU race
		// where a concurrent Delete could remove the file between index lookup and file read.
		webhook, err = s.loadWebhookFromFile(filePath)
	}
	s.mu.RUnlock()

	if err != nil {
		// File might have been deleted externally
		if errors.Is(err, os.ErrNotExist) {
			return nil, auth.ErrWebhookNotFound
		}
		return nil, fmt.Errorf("filewebhook: failed to load webhook %s: %w", id, err)
	}

	return webhook, nil
}

// GetByDAGName retrieves the webhook for a specific DAG.
func (s *Store) GetByDAGName(ctx context.Context, dagName string) (*auth.Webhook, error) {
	if dagName == "" {
		return nil, auth.ErrInvalidWebhookDAGName
	}

	s.mu.RLock()
	webhookID, exists := s.byDAGName[dagName]
	s.mu.RUnlock()

	if !exists {
		return nil, auth.ErrWebhookNotFound
	}

	return s.GetByID(ctx, webhookID)
}

// List returns all webhooks in the store.
func (s *Store) List(ctx context.Context) ([]*auth.Webhook, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	webhooks := make([]*auth.Webhook, 0, len(ids))
	for _, id := range ids {
		webhook, err := s.GetByID(ctx, id)
		if err != nil {
			// Skip webhooks that can't be loaded
			if errors.Is(err, auth.ErrWebhookNotFound) {
				continue
			}
			return nil, err
		}
		webhooks = append(webhooks, webhook)
	}

	return webhooks, nil
}

// Update modifies an existing webhook.
func (s *Store) Update(_ context.Context, webhook *auth.Webhook) error {
	if webhook == nil {
		return errors.New("filewebhook: webhook cannot be nil")
	}
	if webhook.ID == "" {
		return auth.ErrInvalidWebhookID
	}
	if webhook.DAGName == "" {
		return auth.ErrInvalidWebhookDAGName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[webhook.ID]
	if !exists {
		return auth.ErrWebhookNotFound
	}

	// Load existing webhook to check for DAG name change
	existingWebhook, err := s.loadWebhookFromFile(filePath)
	if err != nil {
		return fmt.Errorf("filewebhook: failed to load existing webhook: %w", err)
	}

	// If DAG name changed, check for conflicts (shouldn't normally happen)
	if existingWebhook.DAGName != webhook.DAGName {
		if existingID, taken := s.byDAGName[webhook.DAGName]; taken && existingID != webhook.ID {
			return auth.ErrWebhookAlreadyExists
		}
	}

	// Write updated webhook FIRST, before updating index.
	// This ensures index is only updated on successful write,
	// avoiding corruption if write fails.
	if err := s.writeWebhookToFile(filePath, webhook); err != nil {
		return err
	}

	// Invalidate cache after successful write
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	// Update index AFTER successful file write
	if existingWebhook.DAGName != webhook.DAGName {
		delete(s.byDAGName, existingWebhook.DAGName)
		s.byDAGName[webhook.DAGName] = webhook.ID
	}

	return nil
}

// Delete removes a webhook by its ID.
func (s *Store) Delete(_ context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidWebhookID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return auth.ErrWebhookNotFound
	}

	// Load webhook to get DAG name for index cleanup
	webhook, err := s.loadWebhookFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filewebhook: failed to load webhook for deletion: %w", err)
	}

	// Remove file
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filewebhook: failed to delete webhook file: %w", err)
	}

	// Invalidate cache after file removal
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	// Update index
	delete(s.byID, id)
	if webhook != nil {
		delete(s.byDAGName, webhook.DAGName)
	} else {
		// File was already gone; find DAG name entry that still points to this ID.
		// Note: We must find the key first, then delete after the loop
		// to avoid undefined behavior from modifying a map during iteration.
		var dagNameToDelete string
		for dagName, webhookID := range s.byDAGName {
			if webhookID == id {
				dagNameToDelete = dagName
				break
			}
		}
		if dagNameToDelete != "" {
			delete(s.byDAGName, dagNameToDelete)
		}
	}

	return nil
}

// DeleteByDAGName removes a webhook by its DAG name.
func (s *Store) DeleteByDAGName(ctx context.Context, dagName string) error {
	if dagName == "" {
		return auth.ErrInvalidWebhookDAGName
	}

	s.mu.RLock()
	webhookID, exists := s.byDAGName[dagName]
	s.mu.RUnlock()

	if !exists {
		return auth.ErrWebhookNotFound
	}

	return s.Delete(ctx, webhookID)
}

// UpdateLastUsed updates the LastUsedAt timestamp for a webhook.
func (s *Store) UpdateLastUsed(_ context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidWebhookID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return auth.ErrWebhookNotFound
	}

	// Load existing webhook
	webhook, err := s.loadWebhookFromFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return auth.ErrWebhookNotFound
		}
		return fmt.Errorf("filewebhook: failed to load webhook: %w", err)
	}

	// Update timestamp
	now := time.Now().UTC()
	webhook.LastUsedAt = &now

	// Write updated webhook
	if err := s.writeWebhookToFile(filePath, webhook); err != nil {
		return err
	}

	// Invalidate cache after successful write
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	return nil
}
