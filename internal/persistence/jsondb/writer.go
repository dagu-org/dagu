package jsondb

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/persistence"
)

// WriterState represents the current state of a writer
type WriterState int

const (
	WriterStateClosed WriterState = iota
	WriterStateOpen
)

// Error definitions
var (
	ErrWriterClosed  = errors.New("writer is closed")
	ErrWriterNotOpen = errors.New("writer is not open")
)

// Writer manages writing status to a local file.
// The name is capitalized to make it a public type, assuming it should be accessible
// outside the package (otherwise, keep it lowercase).
type Writer struct {
	target string
	state  WriterState
	writer *bufio.Writer
	file   *os.File
	mu     sync.Mutex
}

// NewWriter creates a new Writer instance for the specified target file path.
func NewWriter(target string) *Writer {
	return &Writer{
		target: target,
		state:  WriterStateClosed,
	}
}

// Open prepares the writer for writing by creating necessary directories
// and opening the target file.
func (w *Writer) Open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == WriterStateOpen {
		return nil // Already open, no need to reopen
	}

	// Create directories if needed
	dir := filepath.Dir(w.target)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open or create file
	file, err := fileutil.OpenOrCreateFile(w.target)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.target, err)
	}

	w.file = file
	w.writer = bufio.NewWriter(file)
	w.state = WriterStateOpen
	return nil
}

// Write serializes the status to JSON and appends it to the file.
func (w *Writer) Write(st persistence.Status) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state != WriterStateOpen {
		return ErrWriterNotOpen
	}

	// Marshal status to JSON
	jsonBytes, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("failed to marshal status: %w", err)
	}

	// Write JSON line
	if _, err := w.writer.Write(jsonBytes); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	// Add newline
	if err := w.writer.WriteByte('\n'); err != nil {
		return fmt.Errorf("failed to write newline: %w", err)
	}

	// Flush to ensure data is written to the underlying file
	if err := w.writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush data: %w", err)
	}

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// It's safe to call Close multiple times.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == WriterStateClosed {
		return nil // Already closed
	}

	var errs []error

	// Flush any buffered data
	if w.writer != nil {
		if err := w.writer.Flush(); err != nil {
			errs = append(errs, fmt.Errorf("flush error: %w", err))
		}
	}

	// Ensure data is synced to disk
	if w.file != nil {
		if err := w.file.Sync(); err != nil {
			errs = append(errs, fmt.Errorf("sync error: %w", err))
		}

		if err := w.file.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close error: %w", err))
		}
	}

	// Reset writer state
	w.writer = nil
	w.file = nil
	w.state = WriterStateClosed

	// Return combined errors if any
	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}
