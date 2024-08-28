// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

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
