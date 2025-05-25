package localhistory

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
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

// DAGDefinition is the name of the file where the DAG definition is stored.
const DAGDefinition = "dag.json"

var _ models.DAGRunAttempt = (*Attempt)(nil)

// Attempt manages an append-only status file with read, write, and compaction capabilities.
// It provides thread-safe operations and supports metrics collection.
type Attempt struct {
	id        string                                // Run ID
	file      string                                // Path to the status file
	writer    *Writer                               // Writer for appending status updates
	mu        sync.RWMutex                          // Mutex for thread safety
	cache     *fileutil.Cache[*models.DAGRunStatus] // Optional cache for read operations
	isClosing atomic.Bool                           // Flag to prevent writes during Close/Compact
	dag       *digraph.DAG                          // DAG associated with the status file
}

// AttemptOption defines a functional option for configuring an Attempt.
type AttemptOption func(*Attempt)

// WithDAG sets the DAG associated with the Attempt.
// This allows the Attempt to store DAG metadata alongside the status data.
func WithDAG(dag *digraph.DAG) AttemptOption {
	return func(r *Attempt) {
		r.dag = dag
	}
}

// ID implements models.DAGRunAttempt.
func (r *Attempt) ID() string {
	return r.id
}

// NewAttempt creates a new Run for the specified file.
func NewAttempt(file string, cache *fileutil.Cache[*models.DAGRunStatus], opts ...AttemptOption) (*Attempt, error) {
	matches := reRun.FindStringSubmatch(filepath.Base(filepath.Dir(file)))
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid file path for run data: %s", file)
	}
	r := &Attempt{id: matches[2], file: file, cache: cache}
	for _, opt := range opts {
		opt(r)
	}
	return r, nil
}

// Exists returns true if the status file exists.
func (r *Attempt) Exists() bool {
	_, err := os.Stat(r.file)
	return err == nil
}

// ModTime returns the last modification time of the status file.
// This is used to determine when the file was last updated.
func (r *Attempt) ModTime() (time.Time, error) {
	info, err := os.Stat(r.file)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// ReadDAG implements models.DAGRunAttempt.
func (r *Attempt) ReadDAG(ctx context.Context) (*digraph.DAG, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	// Determine the path to the DAG definition file
	dir := filepath.Dir(r.file)
	dagFile := filepath.Join(dir, DAGDefinition)

	// Check if the file exists
	if _, err := os.Stat(dagFile); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("DAG definition file not found: %w", err)
		}
		return nil, fmt.Errorf("failed to access DAG definition file: %w", err)
	}

	// Read the file
	data, err := os.ReadFile(dagFile) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("failed to read DAG definition file: %w", err)
	}

	// Parse the JSON data
	var dag digraph.DAG
	if err := json.Unmarshal(data, &dag); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DAG definition: %w", err)
	}

	return &dag, nil
}

// Open initializes the status file for writing. It returns an error if the file is already open.
// The context can be used to cancel the operation.
func (r *Attempt) Open(ctx context.Context) error {
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
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// If it's a new file, create it
	if r.dag != nil {
		dagJSON, err := json.Marshal(r.dag)
		if err != nil {
			return fmt.Errorf("failed to marshal DAG definition: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, DAGDefinition), dagJSON, 0600); err != nil {
			return fmt.Errorf("failed to write DAG definition: %w", err)
		}
	}

	logger.Debugf(ctx, "Initializing status file: %s", r.file)

	writer := NewWriter(r.file)

	if err := writer.Open(); err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	r.writer = writer
	return nil
}

// Write adds a new status to the file. It returns an error if the file is not open
// or is currently being closed. The context can be used to cancel the operation.
func (r *Attempt) Write(ctx context.Context, status models.DAGRunStatus) error {
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
func (r *Attempt) Close(ctx context.Context) error {
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
func (r *Attempt) Compact(ctx context.Context) error {
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
func (r *Attempt) compactLocked(ctx context.Context) error {
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
func (r *Attempt) ReadStatus(ctx context.Context) (*models.DAGRunStatus, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("%w: %v", ErrContextCanceled, ctx.Err())
	default:
		// Continue with operation
	}

	// Try to use cache first if available
	if r.cache != nil {
		status, cacheErr := r.cache.LoadLatest(r.file, func() (*models.DAGRunStatus, error) {
			r.mu.RLock()
			defer r.mu.RUnlock()
			return r.parseLocked()
		})

		if cacheErr == nil {
			return status, nil
		}
	}

	// Cache miss or disabled, perform a direct read
	r.mu.RLock()
	parsed, parseErr := r.parseLocked()
	r.mu.RUnlock()

	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse status file: %w", parseErr)
	}

	return parsed, nil

}

// parseLocked reads the status file and returns the last valid status.
// Must be called with a lock (read or write) already held.
func (r *Attempt) parseLocked() (*models.DAGRunStatus, error) {
	return ParseStatusFile(r.file)
}

// ParseStatusFile reads the status file and returns the last valid status.
// The bufferSize parameter controls the size of the read buffer.
func ParseStatusFile(file string) (*models.DAGRunStatus, error) {
	f, err := os.Open(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer func() {
		_ = f.Close()
	}()

	var (
		offset int64
		result *models.DAGRunStatus
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
			status, err := models.StatusFromJSON(string(line))
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
