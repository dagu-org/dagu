// Copyright (C) 2024 The Daguflow/Dagu Authors
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
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/daguflow/dagu/internal/persistence/model"
	"github.com/daguflow/dagu/internal/util"
)

// writer manages writing status to a local file.
type writer struct {
	statusFile string
	writer     *bufio.Writer
	file       *os.File
	mu         sync.Mutex
	closed     bool
}

func newWriter(target string) (*writer, error) {
	w := &writer{
		statusFile: target,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// open opens the writer.
func (w *writer) open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(w.statusFile), 0755); err != nil {
		return err
	}

	file, err := util.OpenOrCreateFile(w.statusFile)
	if err != nil {
		return err
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	return nil
}

// write appends the status to the local file.
func (w *writer) write(st *model.Status) error {
	if w.writer == nil {
		return fmt.Errorf("writer is not open")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

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
