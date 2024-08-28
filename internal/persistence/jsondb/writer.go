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
package jsondb

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/persistence/model"
	"github.com/dagu-org/dagu/internal/util"
)

var (
	ErrWriterClosed  = errors.New("writer is closed")
	ErrWriterNotOpen = errors.New("writer is not open")
)

// writer manages writing status to a local file.
type writer struct {
	target  string
	dagFile string
	writer  *bufio.Writer
	file    *os.File
	mu      sync.Mutex
	closed  bool
}

// open opens the writer.
func (w *writer) open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	if err := os.MkdirAll(filepath.Dir(w.target), 0755); err != nil {
		return err
	}

	file, err := util.OpenOrCreateFile(w.target)
	if err != nil {
		return err
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	return nil
}

// write appends the status to the local file.
func (w *writer) write(st *model.Status) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWriterClosed
	}

	if w.writer == nil {
		return ErrWriterNotOpen
	}

	jsonb, err := json.Marshal(st)
	if err != nil {
		return err
	}

	if _, err := w.writer.Write(jsonb); err != nil {
		return err
	}

	if err := w.writer.WriteByte('\n'); err != nil {
		return err
	}

	return w.writer.Flush()
}

// close closes the writer.
func (w *writer) close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return nil
	}

	var err error
	if w.writer != nil {
		err = w.writer.Flush()
	}

	if w.file != nil {
		if syncErr := w.file.Sync(); syncErr != nil && err == nil {
			err = syncErr
		}
		if closeErr := w.file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	w.closed = true
	w.writer = nil
	w.file = nil

	return err
}
