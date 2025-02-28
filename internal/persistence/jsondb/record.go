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
)

// HistoryRecord manages an append-only status file with read, write, and compaction capabilities.
type HistoryRecord struct {
	file      string
	writer    *writer
	mu        sync.RWMutex
	cache     *filecache.Cache[*persistence.Status]
	isClosing atomic.Bool // Used to prevent writes during Close/Compact operations
}

// NewHistoryRecord creates a new HistoryRecord for the specified file with optional caching.
func NewHistoryRecord(file string, cache *filecache.Cache[*persistence.Status]) *HistoryRecord {
	return &HistoryRecord{
		file:  file,
		cache: cache,
	}
}

// Open initializes the status file for writing. It returns an error if the file is already open.
func (hr *HistoryRecord) Open(ctx context.Context) error {
	hr.mu.Lock()
	defer hr.mu.Unlock()

	if hr.writer != nil {
		return fmt.Errorf("status file already open: %w", ErrStatusFileOpen)
	}

	// Ensure the directory exists
	dir := filepath.Dir(hr.file)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	logger.Infof(ctx, "Initializing status file: %s", hr.file)

	writer := newWriter(hr.file)
	if err := writer.open(); err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	hr.writer = writer
	return nil
}

// Write adds a new status record to the file. It returns an error if the file is not open
// or is currently being closed.
func (hr *HistoryRecord) Write(_ context.Context, status persistence.Status) error {
	// Check if we're closing before acquiring the mutex to reduce contention
	if hr.isClosing.Load() {
		return fmt.Errorf("cannot write while file is closing: %w", ErrStatusFileNotOpen)
	}

	hr.mu.Lock()
	defer hr.mu.Unlock()

	if hr.writer == nil {
		return fmt.Errorf("status file not open: %w", ErrStatusFileNotOpen)
	}

	if err := hr.writer.write(status); err != nil {
		return fmt.Errorf("failed to write status: %w", ErrWriteFailed)
	}

	return nil
}

// Close properly closes the status file, performs compaction, and invalidates the cache.
// It's safe to call Close multiple times.
func (hr *HistoryRecord) Close(ctx context.Context) error {
	// Set the closing flag to prevent new writes
	hr.isClosing.Store(true)
	defer hr.isClosing.Store(false)

	hr.mu.Lock()
	defer hr.mu.Unlock()

	if hr.writer == nil {
		return nil
	}

	// Create a copy to avoid nil dereference in deferred function
	w := hr.writer
	hr.writer = nil

	// Attempt to compact the file
	if err := hr.compactLocked(ctx); err != nil {
		logger.Warnf(ctx, "Failed to compact file during close: %v", err)
		// Continue with close even if compaction fails
	}

	// Invalidate the cache
	if hr.cache != nil {
		hr.cache.Invalidate(hr.file)
	}

	// Close the writer
	if err := w.close(); err != nil {
		return fmt.Errorf("failed to close writer: %w", err)
	}

	return nil
}

// Compact performs file compaction to optimize storage and read performance.
// It's safe to call while the file is open or closed.
func (hr *HistoryRecord) Compact(ctx context.Context) error {
	// Set the closing flag to prevent new writes during compaction
	hr.isClosing.Store(true)
	defer hr.isClosing.Store(false)

	hr.mu.Lock()
	defer hr.mu.Unlock()

	return hr.compactLocked(ctx)
}

// compactLocked performs actual compaction with the lock already held
func (hr *HistoryRecord) compactLocked(_ context.Context) error {
	status, err := hr.parseLocked()
	if err == io.EOF {
		return nil // Empty file, nothing to compact
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrCompactFailed, hr.file, err)
	}

	// Create a temporary file in the same directory
	dir := filepath.Dir(hr.file)
	tempFile, err := os.CreateTemp(dir, "compact_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempFilePath := tempFile.Name()

	// Close the temp file so we can use our writer
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Write the compacted data to the temp file
	writer := newWriter(tempFilePath)
	if err := writer.open(); err != nil {
		return fmt.Errorf("failed to open temp file writer: %w", err)
	}

	if err := writer.write(*status); err != nil {
		writer.close() // Best effort close
		if removeErr := os.Remove(tempFilePath); removeErr != nil {
			// Log but continue with the original error
			logger.Errorf(nil, "Failed to remove temp file: %v", removeErr)
		}
		return fmt.Errorf("failed to write compacted data: %w", err)
	}

	if err := writer.close(); err != nil {
		return fmt.Errorf("failed to close temp file writer: %w", err)
	}

	// Use atomic rename for safer file replacement
	// This is atomic on POSIX systems and handled specially on Windows
	if err := safeRename(tempFilePath, hr.file); err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	// Invalidate the cache after successful compaction
	if hr.cache != nil {
		hr.cache.Invalidate(hr.file)
	}

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
func (hr *HistoryRecord) ReadStatus() (*persistence.Status, error) {
	statusFile, err := hr.Read()
	if err != nil {
		return nil, err
	}
	return &statusFile.Status, nil
}

// Read returns the full status file information, including the file path.
func (hr *HistoryRecord) Read() (*persistence.StatusFile, error) {
	// Try to use cache first if available
	if hr.cache != nil {
		status, err := hr.cache.LoadLatest(hr.file, func() (*persistence.Status, error) {
			hr.mu.RLock()
			defer hr.mu.RUnlock()
			return hr.parseLocked()
		})
		if err == nil {
			return persistence.NewStatusFile(hr.file, *status), nil
		}
	}

	// Cache miss or disabled, perform a direct read
	hr.mu.RLock()
	parsed, err := hr.parseLocked()
	hr.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	return persistence.NewStatusFile(hr.file, *parsed), nil
}

// parseLocked reads the status file and returns the last valid status.
// Must be called with a lock (read or write) already held.
func (hr *HistoryRecord) parseLocked() (*persistence.Status, error) {
	f, err := os.Open(hr.file)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer f.Close()

	var (
		offset int64
		result *persistence.Status
	)

	// Create a static buffer to reduce allocations
	buffer := make([]byte, 8192)

	// Read append-only file from the beginning and find the last status
	for {
		line, nextOffset, err := readLineFrom(f, offset, buffer)
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
func readLineFrom(f *os.File, offset int64, buffer []byte) ([]byte, int64, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}

	reader := bufio.NewReaderSize(f, len(buffer))
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
