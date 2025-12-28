package fileapikey

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
	// apiKeyFileExtension is the file extension for API key files.
	apiKeyFileExtension = ".json"
	// apiKeyDirPermissions is the permission mode for the API keys directory.
	apiKeyDirPermissions = 0750
	// apiKeyFilePermissions is the permission mode for API key files.
	apiKeyFilePermissions = 0600
)

// Store implements auth.APIKeyStore using the local filesystem.
// API keys are stored as individual JSON files in the configured directory.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	// mu protects the index maps
	mu sync.RWMutex
	// byID maps API key ID to file path
	byID map[string]string
	// byName maps API key name to key ID
	byName map[string]string

	// fileCache caches API key objects to avoid repeated file reads
	fileCache *fileutil.Cache[*auth.APIKey]
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// WithFileCache sets the file cache for API key objects.
func WithFileCache(cache *fileutil.Cache[*auth.APIKey]) Option {
	return func(s *Store) {
		s.fileCache = cache
	}
}

// New creates a new file-based API key store.
// The baseDir must be non-empty; provided Option functions are applied to the store.
// If baseDir does not exist it is created with directory permissions 0750, and an initial
// in-memory index is built from existing API key files.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("fileapikey: baseDir cannot be empty")
	}

	store := &Store{
		baseDir: baseDir,
		byID:    make(map[string]string),
		byName:  make(map[string]string),
	}

	for _, opt := range opts {
		opt(store)
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(baseDir, apiKeyDirPermissions); err != nil {
		return nil, fmt.Errorf("fileapikey: failed to create directory %s: %w", baseDir, err)
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("fileapikey: failed to build index: %w", err)
	}

	return store, nil
}

// rebuildIndex scans the directory and rebuilds the in-memory index.
func (s *Store) rebuildIndex() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing index
	s.byID = make(map[string]string)
	s.byName = make(map[string]string)

	// Scan directory for API key files
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != apiKeyFileExtension {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		apiKey, err := s.loadAPIKeyFromFile(filePath)
		if err != nil {
			// Log warning but continue - don't fail entire index for one bad file
			slog.Warn("Failed to load API key file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[apiKey.ID] = filePath
		s.byName[apiKey.Name] = apiKey.ID
	}

	return nil
}

// loadAPIKeyFromFile reads and parses an API key from a JSON file.
func (s *Store) loadAPIKeyFromFile(filePath string) (*auth.APIKey, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally from baseDir + keyID
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var stored auth.APIKeyForStorage
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("failed to parse API key file %s: %w", filePath, err)
	}

	return stored.ToAPIKey(), nil
}

// apiKeyFilePath returns the file path for an API key ID.
func (s *Store) apiKeyFilePath(keyID string) string {
	return filepath.Join(s.baseDir, keyID+apiKeyFileExtension)
}

// Create stores a new API key.
func (s *Store) Create(_ context.Context, key *auth.APIKey) error {
	if key == nil {
		return errors.New("fileapikey: API key cannot be nil")
	}
	if key.ID == "" {
		return auth.ErrInvalidAPIKeyID
	}
	if key.Name == "" {
		return auth.ErrInvalidAPIKeyName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if name already exists
	if _, exists := s.byName[key.Name]; exists {
		return auth.ErrAPIKeyAlreadyExists
	}

	// Check if ID already exists (shouldn't happen with UUIDs, but be safe)
	if _, exists := s.byID[key.ID]; exists {
		return auth.ErrAPIKeyAlreadyExists
	}

	// Write API key to file
	filePath := s.apiKeyFilePath(key.ID)
	if err := s.writeAPIKeyToFile(filePath, key); err != nil {
		return err
	}

	// Update index
	s.byID[key.ID] = filePath
	s.byName[key.Name] = key.ID

	return nil
}

// writeAPIKeyToFile writes an API key to a JSON file atomically.
func (s *Store) writeAPIKeyToFile(filePath string, key *auth.APIKey) error {
	data, err := json.MarshalIndent(key.ToStorage(), "", "  ")
	if err != nil {
		return fmt.Errorf("fileapikey: failed to marshal API key: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, apiKeyFilePermissions); err != nil {
		return fmt.Errorf("fileapikey: failed to write file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempPath)
		return fmt.Errorf("fileapikey: failed to rename file %s: %w", filePath, err)
	}

	return nil
}

// GetByID retrieves an API key by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*auth.APIKey, error) {
	if id == "" {
		return nil, auth.ErrInvalidAPIKeyID
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	if !exists {
		s.mu.RUnlock()
		return nil, auth.ErrAPIKeyNotFound
	}

	var key *auth.APIKey
	var err error

	// Use cache if available, otherwise load directly
	if s.fileCache != nil {
		key, err = s.fileCache.LoadLatest(filePath, func() (*auth.APIKey, error) {
			return s.loadAPIKeyFromFile(filePath)
		})
	} else {
		// Load file while still holding the read lock to prevent TOCTOU race
		// where a concurrent Delete could remove the file between index lookup and file read.
		key, err = s.loadAPIKeyFromFile(filePath)
	}
	s.mu.RUnlock()

	if err != nil {
		// File might have been deleted externally
		if errors.Is(err, os.ErrNotExist) {
			return nil, auth.ErrAPIKeyNotFound
		}
		return nil, fmt.Errorf("fileapikey: failed to load API key %s: %w", id, err)
	}

	return key, nil
}

// List returns all API keys in the store.
func (s *Store) List(ctx context.Context) ([]*auth.APIKey, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	keys := make([]*auth.APIKey, 0, len(ids))
	for _, id := range ids {
		key, err := s.GetByID(ctx, id)
		if err != nil {
			// Skip keys that can't be loaded
			if errors.Is(err, auth.ErrAPIKeyNotFound) {
				continue
			}
			return nil, err
		}
		keys = append(keys, key)
	}

	return keys, nil
}

// Update modifies an existing API key.
func (s *Store) Update(_ context.Context, key *auth.APIKey) error {
	if key == nil {
		return errors.New("fileapikey: API key cannot be nil")
	}
	if key.ID == "" {
		return auth.ErrInvalidAPIKeyID
	}
	if key.Name == "" {
		return auth.ErrInvalidAPIKeyName
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[key.ID]
	if !exists {
		return auth.ErrAPIKeyNotFound
	}

	// Load existing key to check for name change
	existingKey, err := s.loadAPIKeyFromFile(filePath)
	if err != nil {
		return fmt.Errorf("fileapikey: failed to load existing API key: %w", err)
	}

	// If name changed, check for conflicts (but don't update index yet)
	if existingKey.Name != key.Name {
		if existingID, taken := s.byName[key.Name]; taken && existingID != key.ID {
			return auth.ErrAPIKeyAlreadyExists
		}
	}

	// Write updated API key FIRST, before updating index.
	// This ensures index is only updated on successful write,
	// avoiding corruption if write fails.
	if err := s.writeAPIKeyToFile(filePath, key); err != nil {
		return err
	}

	// Invalidate cache after successful write
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	// Update index AFTER successful file write
	if existingKey.Name != key.Name {
		delete(s.byName, existingKey.Name)
		s.byName[key.Name] = key.ID
	}

	return nil
}

// Delete removes an API key by its ID.
func (s *Store) Delete(_ context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidAPIKeyID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return auth.ErrAPIKeyNotFound
	}

	// Load API key to get name for index cleanup
	key, err := s.loadAPIKeyFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileapikey: failed to load API key for deletion: %w", err)
	}

	// Remove file
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("fileapikey: failed to delete API key file: %w", err)
	}

	// Invalidate cache after file removal
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	// Update index
	delete(s.byID, id)
	if key != nil {
		delete(s.byName, key.Name)
	} else {
		// File was already gone; find name entry that still points to this ID.
		// Note: We must find the key first, then delete after the loop
		// to avoid undefined behavior from modifying a map during iteration.
		var nameToDelete string
		for name, keyID := range s.byName {
			if keyID == id {
				nameToDelete = name
				break
			}
		}
		if nameToDelete != "" {
			delete(s.byName, nameToDelete)
		}
	}

	return nil
}

// UpdateLastUsed updates the LastUsedAt timestamp for an API key.
func (s *Store) UpdateLastUsed(_ context.Context, id string) error {
	if id == "" {
		return auth.ErrInvalidAPIKeyID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return auth.ErrAPIKeyNotFound
	}

	// Load existing key
	key, err := s.loadAPIKeyFromFile(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return auth.ErrAPIKeyNotFound
		}
		return fmt.Errorf("fileapikey: failed to load API key: %w", err)
	}

	// Update timestamp
	now := time.Now().UTC()
	key.LastUsedAt = &now

	// Write updated key
	if err := s.writeAPIKeyToFile(filePath, key); err != nil {
		return err
	}

	// Invalidate cache after successful write
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	return nil
}
