package config

import "sync"

// globalViperMu serializes all access to the shared viper instance used across the application.
// Uses RWMutex to allow concurrent reads while serializing writes.
var globalViperMu sync.RWMutex

// lockViper acquires the global viper write lock (private).
func lockViper() {
	globalViperMu.Lock()
}

// unlockViper releases the global viper write lock (private).
func unlockViper() {
	globalViperMu.Unlock()
}

// WithViperLock runs the provided function while holding the global viper write lock.
// Use this for any operations that modify viper state (Set, BindPFlag, ReadInConfig, etc.).
func WithViperLock(fn func()) {
	lockViper()
	defer unlockViper()
	fn()
}

// WithViperRLock runs the provided function while holding the global viper read lock.
// Use this for read-only operations (Get, IsSet, etc.) to allow concurrent access.
func WithViperRLock(fn func()) {
	globalViperMu.RLock()
	defer globalViperMu.RUnlock()
	fn()
}
