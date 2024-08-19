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
	statusFile string        // Path to the status file
	writer     *bufio.Writer // Buffered writer for efficient writing
	file       *os.File      // File handle
	mu         sync.Mutex    // Mutex for thread-safe operations
	closed     bool          // Flag to indicate if the writer is closed
}

// newWriter creates and initializes a new writer instance.
// It opens the file for writing and returns the writer.
func newWriter(statusFile string) (*writer, error) {
	w := &writer{
		statusFile: statusFile,
	}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

// open prepares the writer for writing by creating necessary directories
// and opening the file.
func (w *writer) open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(w.statusFile), 0755); err != nil {
		return err
	}

	// Open or create the file
	file, err := util.OpenOrCreateFile(w.statusFile)
	if err != nil {
		return err
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	return nil
}

// write appends the status to the local file in JSON format.
// It ensures thread-safety and flushes the buffer after writing.
func (w *writer) write(st *model.Status) error {
	if w.writer == nil {
		return fmt.Errorf("writer is not open")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Convert status to JSON
	jsonb, err := json.Marshal(st)
	if err != nil {
		return err
	}

	// Write JSON data
	if _, err := w.writer.Write(jsonb); err != nil {
		return err
	}

	// Add a newline after each JSON object
	if err := w.writer.WriteByte('\n'); err != nil {
		return err
	}

	// Flush the buffer to ensure data is written to disk
	return w.writer.Flush()
}

// close properly closes the writer, flushing any remaining data
// and closing the file handle.
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
		// Ensure data is synced to disk
		if syncErr := w.file.Sync(); syncErr != nil && err == nil {
			err = syncErr
		}
		// Close the file handle
		if closeErr := w.file.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	w.closed = true
	w.writer = nil
	w.file = nil

	return err
}
