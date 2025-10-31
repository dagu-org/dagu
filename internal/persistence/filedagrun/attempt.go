package filedagrun

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
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

// CancelRequestedFlag is a special flag used to indicate that a cancel request has been made.
const CancelRequestedFlag = "CANCEL_REQUESTED"

var _ execution.DAGRunAttempt = (*Attempt)(nil)

// Attempt manages an append-only status file with read, write, and compaction capabilities.
// It provides thread-safe operations and supports metrics collection.
type Attempt struct {
	id        string                                   // Attempt ID, extracted from the file path
	file      string                                   // Path to the status file
	writer    *Writer                                  // Writer for appending status updates
	mu        sync.RWMutex                             // Mutex for thread safety
	cache     *fileutil.Cache[*execution.DAGRunStatus] // Optional cache for read operations
	isClosing atomic.Bool                              // Flag to prevent writes during Close/Compact
	dag       *core.DAG                                // DAG associated with the status file
}

// AttemptOption defines a functional option for configuring an Attempt.
type AttemptOption func(*Attempt)

// WithDAG sets the DAG associated with the Attempt.
// This allows the Attempt to store DAG metadata alongside the status data.
func WithDAG(dag *core.DAG) AttemptOption {
	return func(att *Attempt) {
		att.dag = dag
	}
}

// ID implements models.DAGRunAttempt.
func (att *Attempt) ID() string {
	return att.id
}

// NewAttempt creates a new Run for the specified file.
func NewAttempt(file string, cache *fileutil.Cache[*execution.DAGRunStatus], opts ...AttemptOption) (*Attempt, error) {
	dirName := filepath.Base(filepath.Dir(file))
	matches := reAttemptDir.FindStringSubmatch(dirName)
	if len(matches) != 3 {
		return nil, fmt.Errorf("invalid file path for run data: %s", file)
	}
	att := &Attempt{id: matches[2], file: file, cache: cache}
	for _, opt := range opts {
		opt(att)
	}

	return att, nil
}

// Exists returns true if the status file exists.
func (att *Attempt) Exists() bool {
	_, err := os.Stat(att.file)
	return err == nil || !os.IsNotExist(err)
}

// ModTime returns the last modification time of the status file.
// This is used to determine when the file was last updated.
func (att *Attempt) ModTime() (time.Time, error) {
	info, err := os.Stat(att.file)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// ReadDAG implements models.DAGRunAttempt.
func (att *Attempt) ReadDAG(_ context.Context) (*core.DAG, error) {
	// Determine the path to the DAG definition file
	dir := filepath.Dir(att.file)
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
	var dag core.DAG
	if err := json.Unmarshal(data, &dag); err != nil {
		return nil, fmt.Errorf("failed to unmarshal DAG definition: %w", err)
	}

	return &dag, nil
}

// Open initializes the status file for writing. It returns an error if the file is already open.
// The context can be used to cancel the operation.
func (att *Attempt) Open(ctx context.Context) error {
	att.mu.Lock()
	defer att.mu.Unlock()

	if att.writer != nil {
		return fmt.Errorf("status file already open: %w", ErrStatusFileOpen)
	}

	// Ensure the directory exists
	dir := filepath.Dir(att.file)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// If it's a new file, create it
	if att.dag != nil {
		dagJSON, err := json.Marshal(att.dag)
		if err != nil {
			return fmt.Errorf("failed to marshal DAG definition: %w", err)
		}
		if err := os.WriteFile(filepath.Join(dir, DAGDefinition), dagJSON, 0600); err != nil {
			return fmt.Errorf("failed to write DAG definition: %w", err)
		}
	}

	logger.Debugf(ctx, "Initializing status file: %s", att.file)

	writer := NewWriter(att.file)

	if err := writer.Open(); err != nil {
		return fmt.Errorf("failed to open writer: %w", err)
	}

	att.writer = writer
	return nil
}

// Write adds a new status to the file. It returns an error if the file is not open
// or is currently being closed. The context can be used to cancel the operation.
func (att *Attempt) Write(ctx context.Context, status execution.DAGRunStatus) error {
	// Check if we're closing before acquiring the mutex to reduce contention
	if att.isClosing.Load() {
		return fmt.Errorf("cannot write while file is closing: %w", ErrStatusFileNotOpen)
	}

	att.mu.Lock()
	defer att.mu.Unlock()

	if att.writer == nil {
		return fmt.Errorf("status file not open: %w", ErrStatusFileNotOpen)
	}

	if writeErr := att.writer.Write(ctx, status); writeErr != nil {
		return fmt.Errorf("failed to write status: %w", ErrWriteFailed)
	}

	// Invalidate cache after successful write
	if att.cache != nil {
		att.cache.Invalidate(att.file)
	}

	return nil
}

// Close properly closes the status file, performs compaction, and invalidates the cache.
// It's safe to call Close multiple times. The context can be used to cancel the operation.
func (att *Attempt) Close(ctx context.Context) error {
	// Set the closing flag to prevent new writes
	att.isClosing.Store(true)
	defer att.isClosing.Store(false)

	att.mu.Lock()
	defer att.mu.Unlock()

	if att.writer == nil {
		return nil
	}

	// Create a copy to avoid nil dereference in deferred function
	w := att.writer
	att.writer = nil

	// Attempt to compact the file
	if compactErr := att.compactLocked(ctx); compactErr != nil {
		logger.Warnf(ctx, "Failed to compact file during close: %v", compactErr)
		// Continue with close even if compaction fails
	}

	// Invalidate the cache
	if att.cache != nil {
		att.cache.Invalidate(att.file)
	}

	// Close the writer
	if closeErr := w.Close(ctx); closeErr != nil {
		return fmt.Errorf("failed to close writer: %w", closeErr)
	}

	return nil
}

// Compact performs file compaction to optimize storage and read performance.
// It's safe to call while the file is open or closed. The context can be used to cancel the operation.
func (att *Attempt) Compact(ctx context.Context) error {
	// Set the closing flag to prevent new writes during compaction
	att.isClosing.Store(true)
	defer att.isClosing.Store(false)

	att.mu.Lock()
	defer att.mu.Unlock()

	return att.compactLocked(ctx)
}

// compactLocked performs actual compaction with the lock already held
func (att *Attempt) compactLocked(ctx context.Context) error {
	status, err := att.parseLocked()
	if err == io.EOF {
		return nil // Empty file, nothing to compact
	}
	if err != nil {
		return fmt.Errorf("%w: %s: %v", ErrCompactFailed, att.file, err)
	}

	// Create a temporary file in the same directory
	dir := filepath.Dir(att.file)
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
	if err := safeRename(tempFilePath, att.file); err != nil {
		return fmt.Errorf("failed to replace original file: %w", err)
	}

	// Invalidate the cache after successful compaction
	if att.cache != nil {
		att.cache.Invalidate(att.file)
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
func (att *Attempt) ReadStatus(_ context.Context) (*execution.DAGRunStatus, error) {
	// Try to use cache first if available
	if att.cache != nil {
		status, cacheErr := att.cache.LoadLatest(att.file, func() (*execution.DAGRunStatus, error) {
			att.mu.RLock()
			defer att.mu.RUnlock()
			return att.parseLocked()
		})

		if cacheErr == nil {
			return status, nil
		}
	}

	// Cache miss or disabled, perform a direct read
	att.mu.RLock()
	parsed, parseErr := att.parseLocked()
	att.mu.RUnlock()

	if parseErr != nil {
		if errors.Is(parseErr, io.EOF) {
			return nil, execution.ErrCorruptedStatusFile // This means no valid status was found in the file
		}
		return nil, fmt.Errorf("failed to parse status file: %w", parseErr)
	}

	return parsed, nil

}

// parseLocked reads the status file and returns the last valid status.
// Must be called with a lock (read or write) already held.
func (att *Attempt) parseLocked() (*execution.DAGRunStatus, error) {
	return ParseStatusFile(att.file)
}

// ParseStatusFile reads the status file and returns the last valid status.
// The bufferSize parameter controls the size of the read buffer.
func ParseStatusFile(file string) (*execution.DAGRunStatus, error) {
	f, err := os.Open(file) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReadFailed, err)
	}
	defer func() {
		_ = f.Close()
	}()

	var (
		offset int64
		result *execution.DAGRunStatus
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
			status, err := execution.StatusFromJSON(string(line))
			if err == nil {
				result = status
			}
		}
	}
}

// RequestCancel implements models.DAGRunAttempt.
// It creates a flag to indicate that the attempt should be canceled.
func (att *Attempt) RequestCancel(ctx context.Context) error {
	dir := filepath.Dir(att.file)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}
	cancelFile := filepath.Join(dir, CancelRequestedFlag)
	if _, err := os.Stat(cancelFile); err == nil {
		return nil
	}
	if err := os.WriteFile(cancelFile, []byte{}, 0600); err != nil {
		return fmt.Errorf("failed to create cancel request file: %w", err)
	}
	logger.Infof(ctx, "Cancel request created for attempt %s at %s", att.id, cancelFile)
	return nil
}

// CancelRequested checks if a cancel request has been made for this attempt.
func (att *Attempt) CancelRequested(ctx context.Context) (bool, error) {
	cancelFile := filepath.Join(filepath.Dir(att.file), CancelRequestedFlag)
	if _, err := os.Stat(cancelFile); err != nil {
		if os.IsNotExist(err) {
			return false, nil // No cancel request found
		}
		return false, fmt.Errorf("failed to check cancel request file: %w", err)
	}

	logger.Infof(ctx, "Cancel request found for attempt %s at %s", att.id, cancelFile)
	return true, nil
}

// Hidden returns true if the attempt is hidden from normal operations.
func (att *Attempt) Hidden() bool {
	att.mu.RLock()
	defer att.mu.RUnlock()

	// Check if the directory name starts with a dot
	dir := filepath.Dir(att.file)
	baseName := filepath.Base(dir)
	return strings.HasPrefix(baseName, ".")
}

// Hide renames the attempt directory to hide it from normal operations.
// It prefixes the directory name with a dot to make it hidden.
func (att *Attempt) Hide(ctx context.Context) error {
	att.mu.Lock()
	defer att.mu.Unlock()

	// Cannot hide if attempt is open
	if att.writer != nil {
		return fmt.Errorf("cannot hide open attempt: %w", ErrStatusFileOpen)
	}

	// Get current directory path
	currentDir := filepath.Dir(att.file)
	baseName := filepath.Base(currentDir)

	// Check if already hidden (idempotent)
	if strings.HasPrefix(baseName, ".") {
		return nil
	}

	// Add dot prefix to hide the directory
	newBaseName := "." + baseName
	newDir := filepath.Join(filepath.Dir(currentDir), newBaseName)

	// Check if target already exists
	if _, err := os.Stat(newDir); err == nil {
		return fmt.Errorf("target directory already exists: %s", newDir)
	}

	// Perform atomic rename
	if err := safeRename(currentDir, newDir); err != nil {
		return fmt.Errorf("failed to hide attempt: %w", err)
	}

	// Update internal file path
	att.file = filepath.Join(newDir, filepath.Base(att.file))

	// Invalidate cache if present
	if att.cache != nil {
		att.cache.Invalidate(att.file)
	}

	// Log the operation
	logger.Infof(ctx, "Hidden attempt %s: %s -> %s", att.id, currentDir, newDir)

	return nil
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
