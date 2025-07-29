// Package dirlock provides a directory-based locking mechanism for coordinating
// access to shared resources across multiple processes.
package dirlock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
)

// Error types for lock operations
var (
	// ErrLockConflict indicates the lock is held by another process
	ErrLockConflict = errors.New("directory is locked by another process")

	// ErrNotLocked indicates unlock was called but lock is not held
	ErrNotLocked = errors.New("directory is not locked")

	// ErrLockNotHeld indicates operation requires holding the lock
	ErrLockNotHeld = errors.New("lock is not held by this instance")

	// ErrInvalidLockFile indicates the lock file is corrupted
	ErrInvalidLockFile = errors.New("lock file is invalid or corrupted")
)

// DirLock represents a directory lock instance
type DirLock interface {
	// TryLock attempts to acquire lock without blocking
	// Returns ErrLockConflict if lock is held by another process
	TryLock() error

	// Lock acquires lock, blocking until available or context is cancelled
	Lock(ctx context.Context) error

	// Unlock releases the lock
	// Returns error if lock is not held by this instance
	Unlock() error

	// IsLocked checks if directory is currently locked
	// Returns true if locked (by any process)
	IsLocked() bool

	// IsHeldByMe checks if this instance holds the lock
	IsHeldByMe() bool

	// Info returns information about current lock holder
	// Returns nil if not locked
	Info() (*LockInfo, error)

	// Heartbeat updates the lock timestamp to prevent staleness
	// Must be called periodically while holding the lock
	Heartbeat(ctx context.Context) error
}

// LockOptions configures lock behavior
type LockOptions struct {
	// StaleThreshold after which lock is considered stale (default: 30s)
	StaleThreshold time.Duration

	// RetryInterval for lock acquisition attempts (default: 50ms)
	RetryInterval time.Duration
}

// LockInfo contains information about a lock
type LockInfo struct {
	AcquiredAt  time.Time
	LockDirName string
}

// dirLock implements the DirLock interface
type dirLock struct {
	id        string
	targetDir string
	lockPath  string
	opts      *LockOptions
	isHeld    bool
	mu        sync.Mutex
}

// New creates a new directory lock instance
func New(directory string, opts *LockOptions) (DirLock, error) {
	if directory == "" {
		return nil, errors.New("directory cannot be empty")
	}

	// Set default options if not provided
	if opts == nil {
		opts = &LockOptions{}
	}
	if opts.StaleThreshold == 0 {
		opts.StaleThreshold = 30 * time.Second
	}
	if opts.RetryInterval == 0 {
		opts.RetryInterval = 50 * time.Millisecond
	}

	return &dirLock{
		id:        generateID(),
		targetDir: directory,
		opts:      opts,
	}, nil
}

// Heartbeat updates the lock's last heartbeat time to prevent it from being
// considered stale. This is an atomic operation that should be called
// periodically while the lock is held to keep it alive. This removes the old
// lock path and creates a new one with the current timestamp.
func (l *dirLock) Heartbeat(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return ErrNotLocked
	}

	if l.lockPath == "" {
		return errors.New("lock path is empty")
	}

	// Create new lock path with current timestamp
	lockName := fmt.Sprintf(".dagu_lock.%s.%d", l.id, time.Now().UnixNano())
	newLockPath := filepath.Join(l.targetDir, lockName)

	// Create new lock directory first
	err := os.Mkdir(newLockPath, 0700)
	if err != nil {
		return fmt.Errorf("failed to create new lock directory: %w", err)
	}

	// Update the lock path to the new one
	l.lockPath = newLockPath

	// Clean stale locks
	if err := l.cleanStale(); err != nil {
		logger.Errorf(ctx, "Failed to clean stale locks: %v", err)
	}

	return nil
}

// TryLock attempts to acquire lock without blocking
func (l *dirLock) TryLock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.isHeld {
		return nil // Already held by us
	}

	// Clean up any stale locks first
	if err := l.cleanStale(); err != nil {
		return fmt.Errorf("failed to clean stale locks: %w", err)
	}

	// Check for any existing non-stale locks
	entries, err := os.ReadDir(l.targetDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			if !l.isStale(entry.Name()) {
				return ErrLockConflict
			}
		}
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(l.targetDir, 0750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Create lock directory with timestamp only
	lockName := fmt.Sprintf(".dagu_lock.%s.%d", l.id, time.Now().UnixNano())
	l.lockPath = filepath.Join(l.targetDir, lockName)

	err = os.Mkdir(l.lockPath, 0700)
	if err != nil {
		if os.IsExist(err) {
			return ErrLockConflict
		}
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	l.isHeld = true
	return nil
}

// Lock acquires lock, blocking until available or context is cancelled
func (l *dirLock) Lock(ctx context.Context) error {
	// Try once without blocking
	if err := l.TryLock(); err == nil {
		return nil
	}

	// Set up retry with context
	ticker := time.NewTicker(l.opts.RetryInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := l.TryLock(); err == nil {
				return nil
			} else if err != ErrLockConflict {
				return err
			}
			// Continue retrying on lock conflict
		}
	}
}

// Unlock releases the lock
func (l *dirLock) Unlock() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return nil
	}

	// Remove all lock directories belonging to this instance
	entries, err := os.ReadDir(l.targetDir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	var removeErrors []error
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			// Check if this lock belongs to us
			id, err := getID(entry.Name())
			if err != nil {
				continue // Invalid lock file, skip
			}
			if id == l.id {
				// This is our lock, remove it
				lockPath := filepath.Join(l.targetDir, entry.Name())
				if err := os.RemoveAll(lockPath); err != nil && !os.IsNotExist(err) {
					removeErrors = append(removeErrors, fmt.Errorf("failed to remove lock %s: %w", lockPath, err))
				}
			}
		}
	}

	// If we had any errors removing locks, return the first one
	if len(removeErrors) > 0 {
		return removeErrors[0]
	}

	l.isHeld = false
	l.lockPath = ""
	return nil
}

// IsLocked checks if directory is currently locked
func (l *dirLock) IsLocked() bool {
	entries, err := os.ReadDir(l.targetDir)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			// Check if this lock is stale
			lockPath := filepath.Join(l.targetDir, entry.Name())
			if !l.isStale(entry.Name()) {
				return true
			}
			// Clean up stale lock
			_ = os.RemoveAll(lockPath)
		}
	}

	return false
}

// IsHeldByMe checks if this instance holds the lock
func (l *dirLock) IsHeldByMe() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if !l.isHeld {
		return false
	}

	// check if the file exists
	_, err := os.Stat(l.lockPath)
	if os.IsNotExist(err) {
		// If the lock path does not exist, it means it was removed
		l.isHeld = false
		return false
	}

	// check if newer lock exists
	entries, err := os.ReadDir(l.targetDir)
	if err != nil {
		return false
	}

	ourTimestamp, err := getTimestamp(filepath.Base(l.lockPath))
	if err != nil {
		return false // Invalid lock path, assume not held
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			id, err := getID(entry.Name())
			if err != nil {
				continue // Invalid lock file, skip
			}
			if id == l.id {
				continue // This is our lock
			}
			timestamp, err := getTimestamp(entry.Name())
			if err != nil {
				continue // Invalid timestamp, skip
			}
			if timestamp > ourTimestamp {
				// Found a newer lock, we don't hold the lock
				return false
			}
		}
	}

	return true
}

// Info returns information about current lock holder
func (l *dirLock) Info() (*LockInfo, error) {
	entries, err := os.ReadDir(l.targetDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			if !l.isStale(entry.Name()) {
				// Extract timestamp from lock directory name
				// Format: .dagu_lock.<id>.<timestamp>
				timestamp, err := getTimestamp(entry.Name())
				if err != nil {
					continue
				}

				return &LockInfo{
					AcquiredAt:  time.Unix(0, timestamp),
					LockDirName: entry.Name(),
				}, nil
			}
		}
	}

	return nil, nil
}

// ForceUnlock forcibly removes a lock (administrative operation)
func ForceUnlock(directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			lockPath := filepath.Join(directory, entry.Name())
			if err := os.RemoveAll(lockPath); err != nil {
				return fmt.Errorf("failed to remove lock directory %s: %w", lockPath, err)
			}
		}
	}

	return nil
}

// cleanStale removes any stale locks
func (l *dirLock) cleanStale() error {
	entries, err := os.ReadDir(l.targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist yet
		}
		return fmt.Errorf("failed to read directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".dagu_lock.") {
			if l.isStale(entry.Name()) {
				lockPath := filepath.Join(l.targetDir, entry.Name())
				if err := os.RemoveAll(lockPath); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("failed to remove stale lock %s: %w", lockPath, err)
				}
			}
		}
	}

	return nil
}

// isStale checks if a lock directory is stale based on age
func (l *dirLock) isStale(lockDirName string) bool {
	timestamp, err := getTimestamp(lockDirName)
	if err != nil {
		return true // Invalid timestamp
	}

	// Check age
	age := time.Now().UnixNano() - timestamp
	return age > int64(l.opts.StaleThreshold.Nanoseconds())
}

// getTimestamp extracts the timestamp from a lock directory name
func getTimestamp(lockDirName string) (int64, error) {
	// Format: .dagu_lock.<id>.<timestamp>
	parts := strings.Split(lockDirName, ".")
	if len(parts) != 4 {
		return 0, ErrInvalidLockFile
	}

	timestamp, err := strconv.ParseInt(parts[3], 10, 64)
	if err != nil {
		return 0, ErrInvalidLockFile
	}

	return timestamp, nil
}

// getID extracts the identifier from the lock directory name
func getID(lockDirName string) (string, error) {
	// Format: .dagu_lock.<id>.<timestamp>
	parts := strings.Split(lockDirName, ".")
	if len(parts) != 4 {
		return "", ErrInvalidLockFile
	}

	return parts[2], nil
}

func generateID() string {
	// Generate an identifier for this lock instance
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	pid := os.Getpid()
	id := fileutil.SafeName(fmt.Sprintf("%s@%d", host, pid))
	// replace '.' with '_' to avoid issues in directory names
	return strings.ReplaceAll(id, ".", "_")
}
