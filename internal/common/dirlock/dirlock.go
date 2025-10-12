// Package dirlock provides a directory-based locking mechanism for coordinating
// access to shared resources across multiple processes.
package dirlock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	targetDir string
	lockPath  string
	opts      *LockOptions
	isHeld    bool
	mu        sync.Mutex
}

// New creates a new directory lock instance
func New(directory string, opts *LockOptions) DirLock {
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
		targetDir: directory,
		lockPath:  filepath.Join(directory, ".dagu_lock"),
		opts:      opts,
	}
}

// Heartbeat updates the lock's modification time to prevent it from being
// considered stale. This is an atomic operation that should be called
// periodically while the lock is held to keep it alive.
func (l *dirLock) Heartbeat(_ context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return ErrNotLocked
	}

	// Touch the lock directory to update its modification time
	now := time.Now()
	if err := os.Chtimes(l.lockPath, now, now); err != nil {
		return fmt.Errorf("failed to update lock timestamp: %w", err)
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

	// Check if lock exists
	info, err := os.Stat(l.lockPath)
	if err == nil {
		// Lock exists, check if it's stale
		if l.isStaleInfo(info) {
			// Remove stale lock
			if err := os.RemoveAll(l.lockPath); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf("failed to remove stale lock: %w", err)
			}
		} else {
			// Lock is held by another process
			return ErrLockConflict
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check lock status: %w", err)
	}

	// Ensure the target directory exists
	if err := os.MkdirAll(l.targetDir, 0750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Try to create the lock directory
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

	// Remove the lock directory
	if err := os.RemoveAll(l.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock directory: %w", err)
	}

	l.isHeld = false
	return nil
}

// IsLocked checks if directory is currently locked
func (l *dirLock) IsLocked() bool {
	info, err := os.Stat(l.lockPath)
	if err != nil {
		return false
	}

	// Check if lock is stale
	if l.isStaleInfo(info) {
		// Clean up stale lock
		_ = os.RemoveAll(l.lockPath)
		return false
	}

	return true
}

// IsHeldByMe checks if this instance holds the lock
func (l *dirLock) IsHeldByMe() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return false
	}

	// Check if the lock directory still exists
	_, err := os.Stat(l.lockPath)
	if os.IsNotExist(err) {
		// Lock was removed externally
		l.isHeld = false
		return false
	}

	return true
}

// Info returns information about current lock holder
func (l *dirLock) Info() (*LockInfo, error) {
	info, err := os.Stat(l.lockPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get lock info: %w", err)
	}

	if l.isStaleInfo(info) {
		// Lock is stale
		return nil, nil
	}

	return &LockInfo{
		AcquiredAt:  info.ModTime(),
		LockDirName: ".dagu_lock",
	}, nil
}

// ForceUnlock forcibly removes a lock (administrative operation)
func ForceUnlock(directory string) error {
	lockPath := filepath.Join(directory, ".dagu_lock")
	if err := os.RemoveAll(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to force unlock: %w", err)
	}
	return nil
}

// isStaleInfo checks if a lock is stale based on file info
func (l *dirLock) isStaleInfo(info os.FileInfo) bool {
	age := time.Since(info.ModTime())
	return age > l.opts.StaleThreshold
}
