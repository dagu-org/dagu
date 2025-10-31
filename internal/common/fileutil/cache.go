package fileutil

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Entry represents a single cached item with metadata and expiration information
type Entry[T any] struct {
	Data         T
	Size         int64
	LastModified int64
	ExpiresAt    time.Time
}

// Cache implements a generic file caching mechanism with TTL-based expiration.
// It stores entries with metadata like size and modification time to detect changes.
// TODO: Consider replacing this with hashicorp/golang-lru for better performance
// https://github.com/hashicorp/golang-lru
type Cache[T any] struct {
	entries  sync.Map
	capacity int
	ttl      time.Duration
	items    atomic.Int32
	stopCh   chan struct{}
}

// NewCache creates a new cache with the specified capacity and time-to-live duration
func NewCache[T any](cap int, ttl time.Duration) *Cache[T] {
	return &Cache[T]{
		capacity: cap,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
}

// Stop halts the cache eviction process
func (c *Cache[T]) Stop() {
	close(c.stopCh)
}

// StartEviction begins the background process of removing expired items
func (c *Cache[T]) StartEviction(ctx context.Context) {
	go func() {
		timer := time.NewTimer(time.Minute)
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				timer.Reset(time.Minute)
				c.evict()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// evict removes expired and excess entries from the cache
func (c *Cache[T]) evict() {
	c.entries.Range(func(key, value any) bool {
		entry := value.(Entry[T])
		if time.Now().After(entry.ExpiresAt) {
			c.entries.Delete(key)
			c.items.Add(-1)
		}
		return true
	})
	if c.capacity > 0 && int(c.items.Load()) > c.capacity {
		c.entries.Range(func(key, _ any) bool {
			c.items.Add(-1)
			c.entries.Delete(key)
			return int(c.items.Load()) > c.capacity
		})
	}
}

// StopEviction signals the eviction goroutine to stop
func (c *Cache[T]) StopEviction() {
	c.stopCh <- struct{}{}
}

// Store adds or updates an item in the cache with metadata from the file
func (c *Cache[T]) Store(fileName string, data T, fi os.FileInfo) {
	c.items.Add(1)
	c.entries.Store(
		fileName, newEntry(data, fi.Size(), fi.ModTime().Unix(), c.ttl))
}

// Invalidate removes an item from the cache
func (c *Cache[T]) Invalidate(fileName string) {
	c.items.Add(-1)
	c.entries.Delete(fileName)
}

// LoadLatest gets the latest version of an item, loading it if stale or missing
func (c *Cache[T]) LoadLatest(
	filePath string, loader func() (T, error),
) (T, error) {
	stale, lastModified, err := c.IsStale(filePath, c.Entry(filePath))
	if err != nil {
		var zero T
		return zero, err
	}
	if stale {
		data, err := loader()
		if err != nil {
			var zero T
			return zero, err
		}
		c.Store(filePath, data, lastModified)
		return data, nil
	}
	item, _ := c.entries.Load(filePath)
	entry := item.(Entry[T])
	return entry.Data, nil
}

// Entry returns the cached entry for a file, or an empty entry if not found
func (c *Cache[T]) Entry(fileName string) Entry[T] {
	item, ok := c.entries.Load(fileName)
	if !ok {
		return Entry[T]{}
	}
	return item.(Entry[T])
}

// Load retrieves an item from the cache if it exists
func (c *Cache[T]) Load(fileName string) (T, bool) {
	item, ok := c.entries.Load(fileName)
	if !ok {
		var zero T
		return zero, false
	}
	entry := item.(Entry[T])
	return entry.Data, true
}

// IsStale checks if a cached entry is stale compared to the file on disk
// by comparing modification time and size
func (*Cache[T]) IsStale(
	fileName string, entry Entry[T],
) (bool, os.FileInfo, error) {
	fi, err := os.Stat(fileName)
	if err != nil {
		return true, fi, fmt.Errorf("failed to stat file %s: %w", fileName, err)
	}
	t := fi.ModTime().Unix()
	return entry.LastModified < t || entry.Size != fi.Size(), fi, nil
}

// newEntry creates a new cache entry with the provided data and metadata
// It adds random jitter to expiration time to prevent a thundering herd problem
func newEntry[T any](
	data T, size int64, lastModified int64, ttl time.Duration,
) Entry[T] {
	expiresAt := time.Now().Add(ttl)
	// Add random jitter to avoid thundering herd
	randBigInt, err := rand.Int(rand.Reader, big.NewInt(60))
	if err != nil {
		panic(err)
	}
	randInt := int(randBigInt.Int64())
	randMin := time.Duration(randInt) * time.Minute
	expiresAt = expiresAt.Add(randMin)

	return Entry[T]{
		Data:         data,
		Size:         size,
		LastModified: lastModified,
		ExpiresAt:    expiresAt,
	}
}
