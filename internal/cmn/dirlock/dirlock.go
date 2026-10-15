// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package dirlock provides a directory-based locking mechanism for coordinating
// access to shared resources across multiple processes.
package dirlock

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
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

	// OnWait is called once when Lock transitions into retry/wait mode after
	// encountering a lock conflict.
	OnWait func()
}

// LockInfo contains information about a lock
type LockInfo struct {
	AcquiredAt  time.Time
	LockDirName string
}

// dirLock implements the DirLock interface
type dirLock struct {
	targetDir  string
	lockPath   string
	opts       *LockOptions
	isHeld     bool
	fenceToken string // unique token written at acquire time
	mu         sync.Mutex

	// Test seams, called at the named point of a lock takeover when set.
	afterStaleObserved func() // after a stale lock was observed, before the claim
	beforeStaleRemoval func() // after verification under the claim, before the rename
}

var _ DirLock = (*dirLock)(nil)

const (
	// LockDirectoryName is the reserved directory name for lock metadata.
	LockDirectoryName           = ".dagu_lock"
	releasedLockDirectoryPrefix = LockDirectoryName + ".releasing."
	takeoverClaimPrefix         = LockDirectoryName + ".takeover."
	lockOwnerFileName           = "owner"
)

// IsLockDirectoryName reports whether name is reserved for lock metadata.
func IsLockDirectoryName(name string) bool {
	return name == LockDirectoryName ||
		strings.HasPrefix(name, releasedLockDirectoryPrefix) ||
		strings.HasPrefix(name, takeoverClaimPrefix)
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
		lockPath:  filepath.Join(directory, LockDirectoryName),
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

	// Verify we still own the lock by checking the fence token
	tokenPath := filepath.Join(l.lockPath, lockOwnerFileName)
	data, err := fileutil.ReadFile(tokenPath)
	if err != nil {
		l.isHeld = false
		return fmt.Errorf("%w: failed to read lock token: %v", ErrLockNotHeld, err)
	}
	if string(data) != l.fenceToken {
		l.isHeld = false
		return fmt.Errorf("%w: token mismatch (lock was acquired by another process)", ErrLockNotHeld)
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
		if !l.isStaleInfo(info) {
			// Lock is held by another process
			return ErrLockConflict
		}
		return l.takeOverStaleLock(info)
	}
	if !os.IsNotExist(err) {
		if isRetryableLockStateError(err) {
			return ErrLockConflict
		}
		return fmt.Errorf("failed to check lock status: %w", err)
	}
	return l.createLock()
}

// takeOverStaleLock replaces the stale lock TryLock observed as info with a
// lock owned by this instance. The caller must hold l.mu.
//
// Contenders that observed the same stale lock race to create a claim
// directory for it. Only the creator may remove the lock and install its own;
// the others return ErrLockConflict and see the winner's lock on their next
// attempt. Without the claim, a slow contender would rename away the fresh
// lock of a faster one, because the rename acts on the path and not on the
// directory that was observed.
func (l *dirLock) takeOverStaleLock(info os.FileInfo) error {
	identity := lockIdentity(l.lockPath, info)
	if l.afterStaleObserved != nil {
		l.afterStaleObserved()
	}

	claimPath := takeoverClaimPath(l.lockPath, identity)
	if err := l.claimTakeover(claimPath); err != nil {
		return err
	}
	// A crash between creating the lock and this removal leaves an empty
	// claim behind, named after a stale lock that is gone by then. A later
	// successful takeover sweeps such claims, see sweepAbandonedClaims.
	defer func() { _ = removeLockDir(claimPath) }()

	// Check, while holding the claim, that the lock is still the one that was
	// observed: another contender may have completed a takeover and released
	// its claim before ours was created.
	current, err := os.Stat(l.lockPath)
	switch {
	case err == nil:
		if !l.isStaleInfo(current) || lockIdentity(l.lockPath, current) != identity {
			return ErrLockConflict
		}
		if l.beforeStaleRemoval != nil {
			l.beforeStaleRemoval()
		}
		if err := l.removeStaleLock(identity); err != nil {
			return err
		}
	case os.IsNotExist(err):
		// Already gone. Creating the new lock below is still exclusive.
	case isRetryableLockStateError(err):
		return ErrLockConflict
	default:
		return fmt.Errorf("failed to check lock status: %w", err)
	}
	if err := l.createLock(); err != nil {
		return err
	}
	l.sweepAbandonedClaims()
	return nil
}

// sweepAbandonedClaims removes the claim directories next to the lock that are
// older than the stale threshold, whichever stale lock they were made for. It
// runs after a takeover, a rare event, so listing the directory is cheap.
func (l *dirLock) sweepAbandonedClaims() {
	entries, err := os.ReadDir(filepath.Dir(l.lockPath))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), takeoverClaimPrefix) {
			l.expireAbandonedClaim(filepath.Join(filepath.Dir(l.lockPath), entry.Name()))
		}
	}
}

// claimTakeover creates the claim directory for a stale lock. It returns
// ErrLockConflict when another contender holds the claim.
//
// A claim left by a process that crashed during its takeover would block that
// lock forever, so a claim older than the stale threshold is treated as
// abandoned and removed. A takeover takes far less time than the threshold.
// The removal is detection-based like the rest of the recovery: contenders
// racing on the same abandoned claim can remove a fresh claim of another.
// Either way this attempt returns ErrLockConflict; the next one claims anew.
func (l *dirLock) claimTakeover(claimPath string) error {
	err := os.Mkdir(claimPath, 0700)
	switch {
	case err == nil:
		return nil
	case os.IsExist(err):
		l.expireAbandonedClaim(claimPath)
		return ErrLockConflict
	case os.IsNotExist(err), isRetryableLockStateError(err):
		return ErrLockConflict
	default:
		return fmt.Errorf("failed to claim stale lock takeover: %w", err)
	}
}

// expireAbandonedClaim removes the claim directory at claimPath if it is
// older than the stale threshold.
func (l *dirLock) expireAbandonedClaim(claimPath string) {
	info, err := os.Stat(claimPath)
	if err != nil || !l.isStaleInfo(info) {
		return
	}
	_ = removeIfUnchanged(claimPath, func(_ string, moved os.FileInfo) bool {
		return l.isStaleInfo(moved) && moved.ModTime().Equal(info.ModTime())
	})
}

// removeStaleLock removes the lock at l.lockPath, which the caller verified
// under the takeover claim to be the stale lock identified by identity.
//
// The directory is renamed aside and checked once more. If it is not the
// observed one, the stale holder released and another process locked in the
// meantime; it is put back and ErrLockConflict is returned. This window stays
// detection-based, see removeIfUnchanged.
func (l *dirLock) removeStaleLock(identity string) error {
	err := removeIfUnchanged(l.lockPath, func(movedPath string, moved os.FileInfo) bool {
		return l.isStaleInfo(moved) && lockIdentity(movedPath, moved) == identity
	})
	if errors.Is(err, errDirChanged) {
		return ErrLockConflict
	}
	return err
}

// errDirChanged reports that removeIfUnchanged found a different directory
// than the one the caller expected and left it in place.
var errDirChanged = errors.New("directory changed since it was observed")

// removeIfUnchanged renames the directory at path aside, checks the moved
// directory with unchanged and removes it only if that holds. Otherwise it
// returns errDirChanged. A path that does not exist is not an error.
//
// The check cannot be atomic with the rename, so a directory that is not the
// expected one is moved briefly. It is renamed back on a best-effort basis:
// rename replaces an empty directory at path, and fails on a non-empty one, in
// which case the moved directory is dropped. In that window a lock can be
// lost, held twice, or left without an owner file until it goes stale. The
// fence token lets the displaced holder notice the loss.
func removeIfUnchanged(path string, unchanged func(movedPath string, moved os.FileInfo) bool) error {
	releasedPath := releasedLockPath(path)
	if err := fileutil.Rename(path, releasedPath); err != nil {
		switch {
		case os.IsNotExist(err):
			return nil
		case isRetryableLockStateError(err):
			return ErrLockConflict
		default:
			return fmt.Errorf("failed to remove stale directory %q: %w", filepath.Base(path), err)
		}
	}

	moved, err := os.Stat(releasedPath)
	if err != nil || !unchanged(releasedPath, moved) {
		if err := fileutil.Rename(releasedPath, path); err != nil {
			_ = fileutil.RemoveAll(releasedPath)
		}
		return errDirChanged
	}

	// The live name is already free. Cleanup failures should not keep other
	// contenders blocked.
	_ = fileutil.RemoveAll(releasedPath)
	return nil
}

// lockIdentity returns a string that identifies the lock directory at path,
// whose file info is info. It is the owner token when the directory has one.
// A directory without a readable token, left behind by a process that
// crashed between creating it and writing the token, is identified by its
// modification time instead. That time no longer changes once the lock is
// stale, and a replacement lock gets a new one.
func lockIdentity(path string, info os.FileInfo) string {
	data, err := fileutil.ReadFile(filepath.Join(path, lockOwnerFileName))
	if err == nil && len(data) > 0 {
		return "token:" + string(data)
	}
	return "mtime:" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
}

// takeoverClaimPath returns the claim directory for the stale lock identified
// by identity.
func takeoverClaimPath(lockPath, identity string) string {
	sum := sha256.Sum256([]byte(identity))
	return filepath.Join(filepath.Dir(lockPath), takeoverClaimPrefix+hex.EncodeToString(sum[:8]))
}

// createLock creates the lock directory and writes this instance's fence
// token into it. The caller must hold l.mu.
func (l *dirLock) createLock() error {
	// Ensure the target directory exists
	if err := fileutil.MkdirAll(l.targetDir, 0750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Try to create the lock directory
	err := os.Mkdir(l.lockPath, 0700)
	if err != nil {
		if os.IsExist(err) || isRetryableLockStateError(err) {
			return ErrLockConflict
		}
		return fmt.Errorf("failed to create lock directory: %w", err)
	}

	// Write a unique ownership token into the lock directory
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "unknown-host"
	}
	token := newFenceToken(hostname)
	tokenPath := filepath.Join(l.lockPath, lockOwnerFileName)
	if err := os.WriteFile(tokenPath, []byte(token), 0600); err != nil {
		_ = removeLockDir(l.lockPath)
		if isRetryableLockStateError(err) {
			return ErrLockConflict
		}
		return fmt.Errorf("failed to write lock token: %w", err)
	}
	l.fenceToken = token
	l.isHeld = true
	return nil
}

func isRetryableLockStateError(err error) bool {
	if err == nil || runtime.GOOS != "windows" {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "access is denied") ||
		strings.Contains(msg, "cannot access the file") ||
		strings.Contains(msg, "sharing violation") ||
		strings.Contains(msg, "used by another process")
}

// Lock acquires lock, blocking until available or context is cancelled
func (l *dirLock) Lock(ctx context.Context) error {
	// Try once without blocking
	if err := l.TryLock(); err == nil {
		return nil
	} else if err != ErrLockConflict {
		return err
	}
	if l.opts.OnWait != nil {
		l.opts.OnWait()
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

	// Verify we still own the lock before removing
	tokenPath := filepath.Join(l.lockPath, lockOwnerFileName)
	data, err := fileutil.ReadFile(tokenPath)
	if err != nil {
		l.isHeld = false
		return fmt.Errorf("%w: failed to read lock token: %v", ErrLockNotHeld, err)
	}
	if string(data) != l.fenceToken {
		// Lock was taken by another process — do NOT remove it
		l.isHeld = false
		return fmt.Errorf("%w: token mismatch (lock was acquired by another process)", ErrLockNotHeld)
	}

	// We still own the lock — safe to remove.
	if err := removeLockDir(l.lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove lock directory: %w", err)
	}

	l.isHeld = false
	return nil
}

// IsLocked checks if directory is currently locked.
// This is an observe-only operation; stale locks are only cleaned up by TryLock.
func (l *dirLock) IsLocked() bool {
	info, err := os.Stat(l.lockPath)
	if err != nil {
		return false
	}
	return !l.isStaleInfo(info)
}

// IsHeldByMe checks if this instance holds the lock
func (l *dirLock) IsHeldByMe() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.isHeld {
		return false
	}

	// Verify the fence token still matches
	tokenPath := filepath.Join(l.lockPath, lockOwnerFileName)
	data, err := fileutil.ReadFile(tokenPath)
	if err != nil || string(data) != l.fenceToken {
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
		LockDirName: LockDirectoryName,
	}, nil
}

// ForceUnlock forcibly removes a lock (administrative operation)
func ForceUnlock(directory string) error {
	lockPath := filepath.Join(directory, LockDirectoryName)
	if err := removeLockDir(lockPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to force unlock: %w", err)
	}
	return nil
}

func removeLockDir(lockPath string) error {
	releasedPath := releasedLockPath(lockPath)
	if err := fileutil.Rename(lockPath, releasedPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	// The live lock name is released after the rename. Cleanup failures should
	// not keep other lock contenders blocked on a lock this process no longer
	// owns.
	_ = fileutil.RemoveAll(releasedPath)
	return nil
}

func releasedLockPath(lockPath string) string {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("%s.releasing.%d.%s", lockPath, os.Getpid(), hex.EncodeToString(suffix[:]))
	}
	return fmt.Sprintf("%s.releasing.%d.%d", lockPath, os.Getpid(), time.Now().UnixNano())
}

// isStaleInfo checks if a lock is stale based on file info
func (l *dirLock) isStaleInfo(info os.FileInfo) bool {
	age := time.Since(info.ModTime())
	return age > l.opts.StaleThreshold
}

func newFenceToken(hostname string) string {
	var suffix [16]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("%s-%d-%s", hostname, os.Getpid(), hex.EncodeToString(suffix[:]))
	}

	return fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())
}
