package storage

import (
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

// Storage is the interface to save / load / delete arbitrary files.
type Storage struct {
	Dir string
}

// NewStorage creates a new storage.
func NewStorage(dir string) *Storage {
	os.MkdirAll(dir, 0755)
	return &Storage{
		Dir: dir,
	}
}

// List returns a list of files in the storage.
func (s *Storage) List() ([]os.FileInfo, error) {
	f, err := os.Open(s.Dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdir(0)
}

// Save writes the given data to the given file.
func (s *Storage) Save(file string, b []byte) error {
	return os.WriteFile(path.Join(s.Dir, file), b, 0644)
}

// Delete deletes the given file.
func (s *Storage) Delete(file string) error {
	return os.Remove(path.Join(s.Dir, file))
}

// MustRead reads the given file and returns the content.
func (s *Storage) MustRead(file string) []byte {
	b, err := os.ReadFile(path.Join(s.Dir, file))
	utils.LogErr("storage: read file", err)
	return b
}
