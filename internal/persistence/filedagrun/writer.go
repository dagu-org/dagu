package filedagrun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// WriterState represents the current state of a writer
type WriterState int

// WriterState constants
const (
	WriterStateClosed WriterState = iota
	WriterStateOpen
)

// Error definitions
var (
	ErrWriterNotOpen = errors.New("writer is not open")
)

// Writer manages writing status to a local file.
// It provides thread-safe operations and ensures data durability.
type Writer struct {
	target     string        // Path to the target file
	state      WriterState   // Current state of the writer
	writer     *bufio.Writer // Buffered writer for performance
	file       *os.File      // Underlying file handle
	mu         sync.Mutex    // Mutex for thread safety
	bufferSize int           // Size of the write buffer
}

// WriterOption defines functional options for configuring a Writer.
type WriterOption func(*Writer)

// NewWriter creates a new Writer instance for the specified target file path.
func NewWriter(target string, opts ...WriterOption) *Writer {
	w := &Writer{
		target:     target,
		state:      WriterStateClosed,
		bufferSize: 4096, // Default buffer size
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
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
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Open or create file
	file, err := fileutil.OpenOrCreateFile(w.target)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", w.target, err)
	}

	w.file = file
	w.writer = bufio.NewWriterSize(file, w.bufferSize)
	w.state = WriterStateOpen
	return nil
}

// Write serializes the status to JSON and appends it to the file.
// It automatically flushes data to ensure durability.
func (w *Writer) Write(ctx context.Context, st execution.DAGRunStatus) error {
	// Add context info to logs if write fails
	if err := w.write(st); err != nil {
		logger.Error(ctx, "Failed to write status",
			tag.Error(err))
		return err
	}

	return nil
}

func (w *Writer) write(st execution.DAGRunStatus) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var err error

	if w.state != WriterStateOpen {
		err = ErrWriterNotOpen
		return err
	}

	// Marshal status to JSON
	jsonBytes, jsonErr := json.Marshal(st)
	if jsonErr != nil {
		err = fmt.Errorf("failed to marshal status: %w", jsonErr)
		return err
	}

	// Write JSON line
	if _, writeErr := w.writer.Write(jsonBytes); writeErr != nil {
		err = fmt.Errorf("failed to write JSON: %w", writeErr)
		return err
	}

	// Add newline
	if nlErr := w.writer.WriteByte('\n'); nlErr != nil {
		err = fmt.Errorf("failed to write newline: %w", nlErr)
		return err
	}

	// Flush to ensure data is written to the underlying file
	if flushErr := w.writer.Flush(); flushErr != nil {
		err = fmt.Errorf("failed to flush data: %w", flushErr)
		return err
	}

	return nil
}

// Close flushes any buffered data and closes the underlying file.
// It's safe to call close multiple times.
func (w *Writer) Close(ctx context.Context) error {
	// Add context info to logs if close fails
	if err := w.close(); err != nil {
		logger.Error(ctx, "Failed to close writer",
			tag.Error(err))
		return err
	}

	return nil
}

// close flushes any buffered data and closes the underlying file.
func (w *Writer) close() error {
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

// IsOpen returns true if the writer is currently open.
func (w *Writer) IsOpen() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state == WriterStateOpen
}
