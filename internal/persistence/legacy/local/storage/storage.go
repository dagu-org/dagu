package storage

import (
	"os"
	"path"
)

// Storage is a storage for flags.
type Storage struct {
	Dir string
}

var (
	// TODO: use 0600 // nolint: gosec
	defaultPermission os.FileMode = 0744
)

// NewStorage creates a new storage.
func NewStorage(dir string) *Storage {
	_ = os.MkdirAll(dir, defaultPermission)

	return &Storage{
		Dir: dir,
	}
}

// Create creates the given file.
func (s *Storage) Create(file string) error {
	return os.WriteFile(path.Join(s.Dir, file), []byte{}, defaultPermission)
}

// Exists returns true if the given file exists.
func (s *Storage) Exists(file string) bool {
	_, err := os.Stat(path.Join(s.Dir, file))
	return err == nil
}

// Delete deletes the given file.
func (s *Storage) Delete(file string) error {
	return os.Remove(path.Join(s.Dir, file))
}
