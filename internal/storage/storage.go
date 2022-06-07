package storage

import (
	"os"
	"path"
)

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

func (s *Storage) Read(file string) ([]byte, error) {
	b, err := os.ReadFile(path.Join(s.Dir, file))
	if err != nil {
		return nil, err
	}
	return b, nil
}
