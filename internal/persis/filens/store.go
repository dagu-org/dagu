package filens

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

	"github.com/google/uuid"
)

const (
	// namespaceFileExtension is the file extension for namespace files.
	namespaceFileExtension = ".json"
	// namespaceDirPermissions is the permission mode for the namespaces directory.
	namespaceDirPermissions = 0750
	// namespaceFilePermissions is the permission mode for namespace files.
	namespaceFilePermissions = 0600
	// DefaultNamespaceName is the name of the default namespace.
	DefaultNamespaceName = "default"
)

// Store implements namespace storage using the local filesystem.
// Namespaces are stored as individual JSON files in the configured directory.
// Thread-safe through internal locking.
type Store struct {
	baseDir string

	// mu protects the index maps
	mu sync.RWMutex
	// byID maps namespace ID to file path
	byID map[string]string
	// byName maps namespace name to namespace ID
	byName map[string]string
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// New creates a new file-based namespace store.
// The baseDir must be non-empty; provided Option functions are applied to the store.
// If baseDir does not exist it is created with directory permissions 0750, and an initial
// in-memory index is built from existing namespace files. Returns an error on invalid input,
// failure to create the directory, or failure to build the initial index.
func New(baseDir string, opts ...Option) (*Store, error) {
	if baseDir == "" {
		return nil, errors.New("filens: baseDir cannot be empty")
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
	if err := os.MkdirAll(baseDir, namespaceDirPermissions); err != nil {
		return nil, fmt.Errorf("filens: failed to create directory %s: %w", baseDir, err)
	}

	// Build initial index
	if err := store.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("filens: failed to build index: %w", err)
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

	// Scan directory for namespace files
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", s.baseDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != namespaceFileExtension {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		ns, err := s.loadNamespaceFromFile(filePath)
		if err != nil {
			// Log warning but continue - don't fail entire index for one bad file
			slog.Warn("Failed to load namespace file during index rebuild",
				slog.String("file", filePath),
				slog.String("error", err.Error()))
			continue
		}

		s.byID[ns.ID] = filePath
		s.byName[ns.Name] = ns.ID
	}

	return nil
}

// loadNamespaceFromFile reads and parses a namespace from a JSON file.
func (s *Store) loadNamespaceFromFile(filePath string) (*Namespace, error) {
	data, err := os.ReadFile(filePath) //nolint:gosec // filePath is constructed internally
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	var ns Namespace
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("failed to parse namespace file %s: %w", filePath, err)
	}

	return &ns, nil
}

// namespaceFilePath returns the file path for a namespace ID.
func (s *Store) namespaceFilePath(nsID string) string {
	return filepath.Join(s.baseDir, nsID+namespaceFileExtension)
}

// Create stores a new namespace.
func (s *Store) Create(_ context.Context, ns *Namespace) error {
	if ns == nil {
		return errors.New("filens: namespace cannot be nil")
	}

	// Generate ID if not provided
	if ns.ID == "" {
		ns.ID = uuid.New().String()
	}

	// Validate name
	if err := ValidateName(ns.Name); err != nil {
		return err
	}

	// Set timestamps
	now := time.Now().UTC()
	ns.CreatedAt = now
	ns.UpdatedAt = now

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if name already exists
	if _, exists := s.byName[ns.Name]; exists {
		return ErrNamespaceAlreadyExists
	}

	// Check if ID already exists (shouldn't happen with UUIDs, but be safe)
	if _, exists := s.byID[ns.ID]; exists {
		return ErrNamespaceAlreadyExists
	}

	// Write namespace to file
	filePath := s.namespaceFilePath(ns.ID)
	if err := s.writeNamespaceToFile(filePath, ns); err != nil {
		return err
	}

	// Update index
	s.byID[ns.ID] = filePath
	s.byName[ns.Name] = ns.ID

	return nil
}

// writeNamespaceToFile writes a namespace to a JSON file atomically.
func (s *Store) writeNamespaceToFile(filePath string, ns *Namespace) error {
	data, err := json.MarshalIndent(ns, "", "  ")
	if err != nil {
		return fmt.Errorf("filens: failed to marshal namespace: %w", err)
	}

	// Write to temp file first, then rename for atomicity
	tempPath := filePath + ".tmp"
	if err := os.WriteFile(tempPath, data, namespaceFilePermissions); err != nil {
		return fmt.Errorf("filens: failed to write file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, filePath); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tempPath)
		return fmt.Errorf("filens: failed to rename file %s: %w", filePath, err)
	}

	return nil
}

// GetByID retrieves a namespace by its unique ID.
func (s *Store) GetByID(_ context.Context, id string) (*Namespace, error) {
	if id == "" {
		return nil, ErrInvalidNamespaceID
	}

	s.mu.RLock()
	filePath, exists := s.byID[id]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrNamespaceNotFound
	}

	ns, err := s.loadNamespaceFromFile(filePath)
	if err != nil {
		// File might have been deleted externally
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNamespaceNotFound
		}
		return nil, fmt.Errorf("filens: failed to load namespace %s: %w", id, err)
	}

	return ns, nil
}

// GetByName retrieves a namespace by its name.
func (s *Store) GetByName(ctx context.Context, name string) (*Namespace, error) {
	if name == "" {
		return nil, ErrInvalidNamespaceName
	}

	s.mu.RLock()
	nsID, exists := s.byName[name]
	s.mu.RUnlock()

	if !exists {
		return nil, ErrNamespaceNotFound
	}

	return s.GetByID(ctx, nsID)
}

// List returns all namespaces in the store.
func (s *Store) List(ctx context.Context) ([]*Namespace, error) {
	s.mu.RLock()
	ids := make([]string, 0, len(s.byID))
	for id := range s.byID {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	namespaces := make([]*Namespace, 0, len(ids))
	for _, id := range ids {
		ns, err := s.GetByID(ctx, id)
		if err != nil {
			// Skip namespaces that can't be loaded
			if errors.Is(err, ErrNamespaceNotFound) {
				continue
			}
			return nil, err
		}
		namespaces = append(namespaces, ns)
	}

	return namespaces, nil
}

// Update modifies an existing namespace.
// Note: This does not allow changing the name - use Rename for that.
func (s *Store) Update(_ context.Context, ns *Namespace) error {
	if ns == nil {
		return errors.New("filens: namespace cannot be nil")
	}
	if ns.ID == "" {
		return ErrInvalidNamespaceID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[ns.ID]
	if !exists {
		return ErrNamespaceNotFound
	}

	// Load existing namespace to preserve name
	existingNs, err := s.loadNamespaceFromFile(filePath)
	if err != nil {
		return fmt.Errorf("filens: failed to load existing namespace: %w", err)
	}

	// Preserve name (use Rename to change name)
	ns.Name = existingNs.Name
	ns.CreatedAt = existingNs.CreatedAt
	ns.CreatedBy = existingNs.CreatedBy
	ns.UpdatedAt = time.Now().UTC()

	// Write updated namespace
	if err := s.writeNamespaceToFile(filePath, ns); err != nil {
		return err
	}

	return nil
}

// Rename changes the name of an existing namespace.
// This only updates the metadata - storage paths use the internal ID.
func (s *Store) Rename(_ context.Context, id, newName string) error {
	if id == "" {
		return ErrInvalidNamespaceID
	}
	if err := ValidateName(newName); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return ErrNamespaceNotFound
	}

	// Check if new name already exists
	if existingID, taken := s.byName[newName]; taken && existingID != id {
		return ErrNamespaceAlreadyExists
	}

	// Load existing namespace
	ns, err := s.loadNamespaceFromFile(filePath)
	if err != nil {
		return fmt.Errorf("filens: failed to load namespace for rename: %w", err)
	}

	oldName := ns.Name
	ns.Name = newName
	ns.UpdatedAt = time.Now().UTC()

	// Write updated namespace
	if err := s.writeNamespaceToFile(filePath, ns); err != nil {
		return err
	}

	// Update index
	delete(s.byName, oldName)
	s.byName[newName] = id

	return nil
}

// Delete removes a namespace by its ID.
// The caller should ensure the namespace is empty before calling this.
func (s *Store) Delete(_ context.Context, id string) error {
	if id == "" {
		return ErrInvalidNamespaceID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	filePath, exists := s.byID[id]
	if !exists {
		return ErrNamespaceNotFound
	}

	// Load namespace to get name for index cleanup
	ns, err := s.loadNamespaceFromFile(filePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filens: failed to load namespace for deletion: %w", err)
	}

	// Remove file
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("filens: failed to delete namespace file: %w", err)
	}

	// Update index
	delete(s.byID, id)
	if ns != nil {
		delete(s.byName, ns.Name)
	} else {
		// File was already gone; remove any name entry that still points to this ID.
		for name, nsID := range s.byName {
			if nsID == id {
				delete(s.byName, name)
				break
			}
		}
	}

	return nil
}

// Count returns the total number of namespaces.
func (s *Store) Count(_ context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return int64(len(s.byID)), nil
}

// Exists checks if a namespace with the given name exists.
func (s *Store) Exists(_ context.Context, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.byName[name]
	return exists
}

// GetIDByName returns the internal ID for a namespace name.
// Returns empty string if not found.
func (s *Store) GetIDByName(_ context.Context, name string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.byName[name]
}

// EnsureDefault creates the default namespace if it doesn't exist.
// Returns the default namespace (existing or newly created).
func (s *Store) EnsureDefault(ctx context.Context, createdBy string) (*Namespace, error) {
	// Check if default already exists
	ns, err := s.GetByName(ctx, DefaultNamespaceName)
	if err == nil {
		return ns, nil
	}
	if !errors.Is(err, ErrNamespaceNotFound) {
		return nil, err
	}

	// Create default namespace
	ns = &Namespace{
		Name:        DefaultNamespaceName,
		DisplayName: "Default",
		Description: "Default namespace for DAGs",
		CreatedBy:   createdBy,
	}

	if err := s.Create(ctx, ns); err != nil {
		// Handle race condition where another process created it
		if errors.Is(err, ErrNamespaceAlreadyExists) {
			return s.GetByName(ctx, DefaultNamespaceName)
		}
		return nil, err
	}

	return ns, nil
}
