package filebaseconfig

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core/baseconfig"
)

// Verify Store implements baseconfig.Store at compile time.
var _ baseconfig.Store = (*Store)(nil)

const filePermissions = 0600

// Store implements a file-based store for the base DAG configuration.
// Thread-safe through internal locking.
type Store struct {
	filePath  string
	mu        sync.RWMutex
	fileCache *fileutil.Cache[string]
}

// Option is a functional option for configuring the Store.
type Option func(*Store)

// WithFileCache sets the file cache for the store.
func WithFileCache(cache *fileutil.Cache[string]) Option {
	return func(s *Store) {
		s.fileCache = cache
	}
}

// New creates a new file-based base config store.
// The filePath is the full path to the base.yaml file.
func New(filePath string, opts ...Option) (*Store, error) {
	if filePath == "" {
		return nil, errors.New("filebaseconfig: filePath cannot be empty")
	}

	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return nil, fmt.Errorf("filebaseconfig: failed to create directory %s: %w", dir, err)
	}

	s := &Store{filePath: filePath}
	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

// GetSpec returns the raw YAML content of the base configuration.
// Returns an empty string if the file does not exist.
func (s *Store) GetSpec(_ context.Context) (string, error) {
	if s.fileCache != nil {
		return s.fileCache.LoadLatest(s.filePath, s.readFromFile)
	}
	return s.readFromFile()
}

// readFromFile reads the config file directly from disk.
func (s *Store) readFromFile() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("filebaseconfig: failed to read file: %w", err)
	}

	return string(data), nil
}

// UpdateSpec writes the given YAML content to the base configuration file.
func (s *Store) UpdateSpec(_ context.Context, spec []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fileutil.WriteFileAtomic(s.filePath, spec, filePermissions); err != nil {
		return fmt.Errorf("filebaseconfig: failed to write file: %w", err)
	}

	if s.fileCache != nil {
		s.fileCache.Invalidate(s.filePath)
	}

	return nil
}
