package jsondb

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
)

// Error definitions for common issues
var (
	ErrStatusFileOpen    = errors.New("status file already open")
	ErrStatusFileNotOpen = errors.New("status file not open")
	ErrReadFailed        = errors.New("failed to read status file")
	ErrWriteFailed       = errors.New("failed to write to status file")
	ErrCompactFailed     = errors.New("failed to compact status file")
	ErrContextCanceled   = errors.New("operation canceled by context")
)

var _ persistence.Record = (*Record)(nil)

// Record manages an append-only status file with read, write, and compaction capabilities.
// It provides thread-safe operations and supports metrics collection.
type Record struct {
	file      string                                // Path to the status file
	writer    *Writer                               // Writer for appending status updates
	mu        sync.RWMutex                          // Mutex for thread safety
	cache     *filecache.Cache[*persistence.Status] // Optional cache for read operations
	isClosing atomic.Bool                           // Flag to prevent writes during Close/Compact
}

// NewRecord creates a new HistoryRecord for the specified file.
func NewRecord(file string, cache *filecache.Cache[*persistence.Status]) *Record {
	return &Record{
		file:  file,
		cache: cache,
	}
}

// Open initializes the status file for writing. It returns an error if the file is already open.
// The context can be used to cancel the operation.
func (r *Record) Open(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer != nil {
		return fmt.Errorf("status file already open: %w", ErrStatusFileOpen)
	}

	// Ensure the directory exists
	dir := filepath.Dir(r.file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	logger.Infof(ctx, "Initializing status file: %s", r.file)

	writer := NewWriter(r.file)

	if err := writer.Open(); err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	r.writer = writer
	return nil
}

// Write adds a new status record to the file. It returns an error if the file is not open
// or is currently being closed. The context can be used to cancel the operation.
func (r *Record) Write(ctx context.Context, status persistence.Status) error {
	// Check if we're closing before acquiring the mutex to reduce contention
	if r.isClosing.Load() {
		return fmt.Errorf("cannot write while file is closing: %w", ErrStatusFileNotOpen)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer == nil {
		return fmt.Errorf("status file not open: %w", ErrStatusFileNotOpen)
	}

	if writeErr := r.writer.Write(ctx, status); writeErr != nil {
		return fmt.Errorf("failed to write status: %w", ErrWriteFailed)
	}

	// Invalidate cache after successful write
	if r.cache != nil {
		r.cache.Invalidate(r.file)
	}

	return nil
}

// Close properly closes the status file, performs compaction, and invalidates the cache.
// It's safe to call Close multiple times. The context can be used to cancel the operation.
func (r *Record) Close(ctx context.Context) error {
	// Set the closing flag to prevent new writes
	r.isClosing.Store(true)
	defer r.isClosing.Store(false)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.writer == nil {
		return nil
	}

	// Create a copy to avoid nil dereference in deferred function
	w := r.writer
	r.writer = nil

	// Attempt to compact the file
	if compactErr := r.compactLocked(ctx); compactErr != nil {
		logger.Warnf(ctx, "Failed to compact file during close: %v", compactErr)
		// Continue with close even if compaction fails
	}

	// Invalidate the cache
	if r.cache != nil {
		r.cache.Invalidate(r.file)
	}

	// Close the writer
	if closeErr := w.Close(ctx); closeErr != nil {
		return fmt.Errorf("failed to close writer: %w", closeErr)
	}

	return nil
}

// Compact performs file compaction to optimize storage and read performance.
// It's safe to call while the file is open or closed. The context can be used to cancel the operation.
func (r *Record) Compact(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	// Set the closing flag to prevent new writes during compaction
	r.isClosing.Store(true)
	defer r.isClosing.Store(false)

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.compactLocked(ctx)
}

// compactLocked performs actual compaction with the lock already held
func (r *Record) compactLocked(ctx context.Context) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	status, err := r.parseLocked()
	if err == io.EOF {
		return nil // Empty file, nothing to compact
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrCompactFailed, r.file, err)
	}

	// Create a temporary file in the same directory
	dir := filepath.Dir(r.file)
	tempFile, err := os.CreateTemp(dir, "compact_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Close the temp file so we can use our writer
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Ensure temp file is cleaned up on error
	success := false
	defer func() {
		if !success {
			if removeErr := os.Remove(tempFilePath); removeErr != nil {
				logger.Errorf(ctx, "Failed to remove temp file: %v", removeErr)
			}
		}
	}()

	// Write the compacted data to the temp file
	writer := NewWriter(tempFilePath)

	if err := writer.Open(); err != nil {
		return fmt.Errorf("failed to open temp file writer: %w", err)
	}

	if err := writer.Write(ctx, *status); err != nil {
		_ = writer.close() // Best effort close
		return fmt.Errorf("failed to write compacted data: %w", err)
	}

	if err := writer.close(); err != nil {
		return fmt.Errorf("failed to close temp file writer: %w", err)
	}

	// Use atomic rename for safer file replacement
	if err := safeRename(tempFilePath, r.file); err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	// Invalidate the cache after successful compaction
	if r.cache != nil {
		r.cache.Invalidate(r.file)
	}

	success = true
	return nil
}

// safeRename safely replaces the target file with the source file,
// handling platform-specific differences
func safeRename(source, target string) error {
	// On Windows, we need to remove the target file first
	if _, err := os.Stat(target); err == nil {
		if err := os.Remove(target); err != nil {
			return fmt.Errorf("failed to remove target file: %w", err)
		}
	}

	return os.Rename(source, target)
}

// ReadStatus reads the latest status from the file, using cache if available.
// The context can be used to cancel the operation.
func (r *Record) ReadStatus(ctx context.Context) (*persistence.Status, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	statusFile, err := r.Read(ctx)
	if err != nil {
		return nil, err
	}
	return &statusFile.Status, nil
}

// Read returns the full status file information, including the file path.
// The context can be used to cancel the operation.
func (r *Record) Read(ctx context.Context) (*persistence.StatusFile, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	// Try to use cache first if available
	if r.cache != nil {
		status, cacheErr := r.cache.LoadLatest(r.file, func() (*persistence.Status, error) {
			r.mu.RLock()
			defer r.mu.RUnlock()
			return r.parseLocked()
		})

		if cacheErr == nil {
			return persistence.NewStatusFile(r.file, *status), nil
		}
	}

	// Cache miss or disabled, perform a direct read
	r.mu.RLock()
	parsed, parseErr := r.parseLocked()
	r.mu.RUnlock()

	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse status file: %w", parseErr)
	}

	return persistence.NewStatusFile(r.file, *parsed), nil
}

// parseLocked reads the status file and returns the last valid status.
// Must be called with a lock (read or write) already held.
func (r *Record) parseLocked() (*persistence.Status, error) {
	return ParseStatusFile(r.file)
}

// ParseStatusFile reads the status file and returns the last valid status.
// The bufferSize parameter controls the size of the read buffer.
func ParseStatusFile(file string) (*persistence.Status, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer f.Close()

	var (
		offset int64
		result *persistence.Status
	)

	// Read append-only file from the beginning and find the last status
	for {
		line, nextOffset, err := readLineFrom(f, offset)
		if err == io.EOF {
			if result == nil {
				return nil, err
			}
			return result, nil
		} else if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
		}

		offset = nextOffset
		if len(line) > 0 {
			status, err := persistence.StatusFromJSON(string(line))
			if err == nil {
				result = status
			}
		}
	}
}

// readLineFrom reads a line from the file starting at the specified offset.
// It returns the line, the new offset, and any error encountered.
// The buffer is used to reduce allocations.
func readLineFrom(f *os.File, offset int64) ([]byte, int64, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	reader := bufio.NewReader(f)
	var line []byte
	var err error

	// Read the line
	line, err = reader.ReadBytes('\n')
	if err != nil && err != io.EOF {
		return nil, offset, err
	}

	// Calculate the new offset
	newOffset := offset + int64(len(line))

	// Trim the newline character if present
	if len(line) > 0 && line[len(line)-1] == '\n' {
		line = line[:len(line)-1]
	}

	return line, newOffset, err
}
