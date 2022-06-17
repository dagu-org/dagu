package storage

import (
	"os"
	"path"

	"github.com/yohamta/dagu/internal/utils"
)

func NewStorage(dir string) *Storage {
	os.MkdirAll(dir, 0755)
	return &Storage{
		Dir: dir,
	}
}

type Storage struct {
	Dir string
}

func (s *Storage) List() ([]os.FileInfo, error) {
	f, err := os.Open(s.Dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return f.Readdir(0)
}

func (s *Storage) Save(file string, b []byte) error {
	return os.WriteFile(path.Join(s.Dir, file), b, 0644)
}

func (s *Storage) Delete(file string) error {
	return os.Remove(path.Join(s.Dir, file))
}

func (s *Storage) MustRead(file string) []byte {
	b, err := os.ReadFile(path.Join(s.Dir, file))
	utils.LogErr("storage: read file", err)
	return b
}
