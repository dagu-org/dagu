package filenamespace

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core/exec"
)

var _ exec.NamespaceStore = (*Store)(nil)

const (
	// DefaultShortID is the well-known short ID for the "default" namespace.
	DefaultShortID = "0000"
	// shortIDLength is the number of hex characters in a short ID.
	shortIDLength = 4
	// maxNameLength is the maximum length of a namespace name.
	maxNameLength = 63
	// filePerm is the file permission for namespace JSON files.
	filePerm = 0600
	// dirPerm is the directory permission for the namespace base directory.
	dirPerm = 0750
)

// nameRegex validates namespace names: [a-z0-9][a-z0-9-]*[a-z0-9], max 63 chars.
// Single character names like "a" are also valid.
var nameRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// Option configures a Store.
type Option func(*Store)

// WithFileCache sets the file cache for namespace objects.
func WithFileCache(cache *fileutil.Cache[*exec.Namespace]) Option {
	return func(s *Store) {
		s.fileCache = cache
	}
}

// Store is a file-based implementation of exec.NamespaceStore.
// Each namespace is persisted as a JSON file ({shortID}.json) under the base directory.
type Store struct {
	baseDir   string
	fileCache *fileutil.Cache[*exec.Namespace]
	mu        sync.RWMutex
	// index maps namespace name -> Namespace for fast lookups.
	index map[string]*exec.Namespace
	// shortIDs maps shortID -> namespace name for collision detection.
	shortIDs map[string]string
}

// New creates a new file-based NamespaceStore.
// The "default" namespace is automatically created if it does not already exist.
func New(baseDir string, opts ...Option) (*Store, error) {
	if err := os.MkdirAll(baseDir, dirPerm); err != nil {
		return nil, fmt.Errorf("failed to create namespace directory %s: %w", baseDir, err)
	}
	s := &Store{
		baseDir:  baseDir,
		index:    make(map[string]*exec.Namespace),
		shortIDs: make(map[string]string),
	}
	for _, opt := range opts {
		opt(s)
	}
	if err := s.rebuildIndex(); err != nil {
		return nil, fmt.Errorf("failed to rebuild namespace index: %w", err)
	}
	if err := s.ensureDefaultNamespace(); err != nil {
		return nil, fmt.Errorf("failed to ensure default namespace: %w", err)
	}
	return s, nil
}

// ensureDefaultNamespace creates the "default" namespace if it does not already exist.
// This runs automatically on startup and is idempotent.
func (s *Store) ensureDefaultNamespace() error {
	if _, exists := s.index["default"]; exists {
		slog.Debug("default namespace already exists, skipping migration")
		return nil
	}

	slog.Info("auto-migration: creating default namespace registry",
		"name", "default",
		"short_id", DefaultShortID,
		"base_dir", s.baseDir,
	)

	ns := &exec.Namespace{
		Name:        "default",
		ShortID:     DefaultShortID,
		CreatedAt:   time.Now(),
		Description: "Default namespace",
	}

	if err := s.writeFile(ns); err != nil {
		return err
	}

	s.index[ns.Name] = ns
	s.shortIDs[ns.ShortID] = ns.Name

	slog.Info("auto-migration: default namespace created successfully",
		"file", s.filePath(DefaultShortID),
	)

	return nil
}

// Create persists a new namespace and returns it.
func (s *Store) Create(_ context.Context, opts exec.CreateNamespaceOptions) (*exec.Namespace, error) {
	if err := validateName(opts.Name); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.index[opts.Name]; exists {
		return nil, exec.ErrNamespaceAlreadyExists
	}

	shortID := generateShortID(opts.Name)

	// Check for hash collision: different name but same short ID.
	if existingName, collision := s.shortIDs[shortID]; collision && existingName != opts.Name {
		return nil, fmt.Errorf("%w: %q and %q both produce short ID %q",
			exec.ErrNamespaceHashCollision, opts.Name, existingName, shortID)
	}

	ns := &exec.Namespace{
		Name:        opts.Name,
		ShortID:     shortID,
		CreatedAt:   time.Now(),
		Description: opts.Description,
		Defaults:    opts.Defaults,
		GitSync:     opts.GitSync,
	}

	if err := s.writeFile(ns); err != nil {
		return nil, err
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(s.filePath(ns.ShortID))
	}

	s.index[ns.Name] = ns
	s.shortIDs[ns.ShortID] = ns.Name

	return ns, nil
}

// Update applies partial updates to an existing namespace.
func (s *Store) Update(_ context.Context, name string, opts exec.UpdateNamespaceOptions) (*exec.Namespace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, exists := s.index[name]
	if !exists {
		return nil, exec.ErrNamespaceNotFound
	}

	if opts.Description != nil {
		ns.Description = *opts.Description
	}
	if opts.Defaults != nil {
		ns.Defaults = *opts.Defaults
	}
	if opts.BaseConfig != nil {
		ns.BaseConfig = opts.BaseConfig
	}
	if opts.BaseConfigYAML != nil {
		ns.BaseConfigYAML = *opts.BaseConfigYAML
	}
	if opts.GitSync != nil {
		ns.GitSync = *opts.GitSync
	}

	if err := s.writeFile(ns); err != nil {
		return nil, err
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(s.filePath(ns.ShortID))
	}

	return ns, nil
}

// Delete removes a namespace by name.
func (s *Store) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	ns, exists := s.index[name]
	if !exists {
		return exec.ErrNamespaceNotFound
	}

	filePath := s.filePath(ns.ShortID)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete namespace file %s: %w", filePath, err)
	}
	if s.fileCache != nil {
		s.fileCache.Invalidate(filePath)
	}

	delete(s.index, name)
	delete(s.shortIDs, ns.ShortID)

	return nil
}

// Get retrieves a namespace by name.
func (s *Store) Get(_ context.Context, name string) (*exec.Namespace, error) {
	s.mu.RLock()
	ns, exists := s.index[name]
	s.mu.RUnlock()

	if !exists {
		return nil, exec.ErrNamespaceNotFound
	}

	if s.fileCache == nil {
		return ns, nil
	}

	filePath := s.filePath(ns.ShortID)
	return s.fileCache.LoadLatest(filePath, func() (*exec.Namespace, error) {
		return s.readFromFile(filePath)
	})
}

// List returns all namespaces.
func (s *Store) List(_ context.Context) ([]*exec.Namespace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*exec.Namespace, 0, len(s.index))
	for _, ns := range s.index {
		result = append(result, ns)
	}

	return result, nil
}

// Resolve returns the short ID for a given namespace name.
func (s *Store) Resolve(_ context.Context, name string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ns, exists := s.index[name]
	if !exists {
		return "", exec.ErrNamespaceNotFound
	}

	return ns.ShortID, nil
}

// generateShortID produces a 4-character hex string from the SHA256 hash of the name.
// The "default" namespace always returns the well-known fixed short ID "0000".
func generateShortID(name string) string {
	if name == "default" {
		return DefaultShortID
	}
	hash := sha256.Sum256([]byte(name))
	return fmt.Sprintf("%x", hash[:2])[:shortIDLength]
}

// validateName checks that a namespace name conforms to the required format.
func validateName(name string) error {
	if len(name) == 0 {
		return fmt.Errorf("namespace name must not be empty")
	}
	if len(name) > maxNameLength {
		return fmt.Errorf("namespace name must be at most %d characters, got %d", maxNameLength, len(name))
	}
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("namespace name %q must match [a-z0-9][a-z0-9-]*[a-z0-9]", name)
	}
	return nil
}

// readFromFile reads and parses a namespace JSON file from disk.
func (s *Store) readFromFile(filePath string) (*exec.Namespace, error) {
	data, err := os.ReadFile(filePath) // #nosec G304 - path constructed from internal baseDir
	if err != nil {
		return nil, fmt.Errorf("failed to read namespace file %s: %w", filePath, err)
	}
	var ns exec.Namespace
	if err := json.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("failed to parse namespace file %s: %w", filePath, err)
	}
	return &ns, nil
}

// writeFile persists a namespace to disk as JSON.
func (s *Store) writeFile(ns *exec.Namespace) error {
	filePath := s.filePath(ns.ShortID)
	return fileutil.WriteJSONAtomic(filePath, ns, filePerm)
}

// filePath returns the JSON file path for a namespace short ID.
func (s *Store) filePath(shortID string) string {
	return filepath.Join(s.baseDir, shortID+".json")
}

// rebuildIndex scans the base directory and loads all namespace JSON files.
func (s *Store) rebuildIndex() error {
	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return fmt.Errorf("failed to read namespace directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(s.baseDir, entry.Name())
		data, err := os.ReadFile(filePath) // #nosec G304 - path constructed from internal baseDir
		if err != nil {
			return fmt.Errorf("failed to read namespace file %s: %w", filePath, err)
		}

		var ns exec.Namespace
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse namespace file %s: %w", filePath, err)
		}

		s.index[ns.Name] = &ns
		s.shortIDs[ns.ShortID] = ns.Name
	}

	return nil
}
