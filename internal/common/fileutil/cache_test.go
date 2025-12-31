package fileutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	assert.Equal(t, "test", cache.Name())
	assert.Equal(t, 0, cache.Size())
	assert.Equal(t, 100, cache.capacity)
	assert.Equal(t, time.Hour, cache.ttl)
}

func TestCache_StoreAndLoad(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	// Create a temp file for testing
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	// Store and verify
	cache.Store(filePath, "test-data", fi)
	assert.Equal(t, 1, cache.Size())

	// Load and verify
	data, ok := cache.Load(filePath)
	assert.True(t, ok)
	assert.Equal(t, "test-data", data)

	// Load non-existent
	_, ok = cache.Load("non-existent")
	assert.False(t, ok)
}

func TestCache_Invalidate(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	cache.Store(filePath, "test-data", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size())

	_, ok := cache.Load(filePath)
	assert.False(t, ok)
}

func TestCache_CapacityLimit(t *testing.T) {
	t.Parallel()

	// Create cache with capacity of 5
	cache := NewCache[string]("test", 5, time.Hour)

	tmpDir := t.TempDir()

	// Add 10 items
	for i := 0; i < 10; i++ {
		filePath := filepath.Join(tmpDir, "test"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		fi, err := os.Stat(filePath)
		require.NoError(t, err)

		cache.Store(filePath, "data", fi)
	}

	// Before eviction, all 10 items are stored
	assert.Equal(t, 10, cache.Size())

	// Trigger eviction
	cache.evict()

	// After eviction, should be exactly at capacity
	assert.Equal(t, 5, cache.Size())
}

func TestCache_TTLExpiration(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	// Directly insert an expired entry to test eviction logic
	// (bypassing the Store method which adds jitter)
	expiredEntry := Entry[string]{
		Data:         "expired-data",
		Size:         10,
		LastModified: time.Now().Unix(),
		ExpiresAt:    time.Now().Add(-time.Hour), // Already expired
	}
	cache.entries.Store("expired-key", expiredEntry)
	cache.items.Add(1)

	assert.Equal(t, 1, cache.Size())

	// Trigger eviction
	cache.evict()

	// Expired item should be evicted
	assert.Equal(t, 0, cache.Size())
}

func TestCache_IsStale(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	cache.Store(filePath, "test-data", fi)

	t.Run("NotStaleWhenUnchanged", func(t *testing.T) {
		entry := cache.Entry(filePath)
		stale, _, err := cache.IsStale(filePath, entry)
		require.NoError(t, err)
		assert.False(t, stale)
	})

	t.Run("StaleWhenModified", func(t *testing.T) {
		// Wait a bit and modify the file
		time.Sleep(10 * time.Millisecond)
		require.NoError(t, os.WriteFile(filePath, []byte("new content"), 0644))

		entry := cache.Entry(filePath)
		stale, _, err := cache.IsStale(filePath, entry)
		require.NoError(t, err)
		assert.True(t, stale)
	})

	t.Run("StaleWhenSizeChanged", func(t *testing.T) {
		// Store with current state
		fi2, _ := os.Stat(filePath)
		cache.Store(filePath, "data", fi2)

		// Change file size
		require.NoError(t, os.WriteFile(filePath, []byte("much longer content here"), 0644))

		entry := cache.Entry(filePath)
		stale, _, err := cache.IsStale(filePath, entry)
		require.NoError(t, err)
		assert.True(t, stale)
	})
}

func TestCache_LoadLatest(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	loadCount := 0
	loader := func() (string, error) {
		loadCount++
		return "loaded-data", nil
	}

	// First call should invoke loader
	data, err := cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 1, loadCount)

	// Second call should use cache (file unchanged)
	data, err = cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 1, loadCount) // Loader not called again

	// Modify file
	time.Sleep(10 * time.Millisecond)
	require.NoError(t, os.WriteFile(filePath, []byte("new content"), 0644))

	// Third call should invoke loader again
	data, err = cache.LoadLatest(filePath, loader)
	require.NoError(t, err)
	assert.Equal(t, "loaded-data", data)
	assert.Equal(t, 2, loadCount)
}

func TestCache_StartEviction(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache.StartEviction(ctx)

	// Verify the eviction goroutine starts without error
	// The actual eviction logic is tested in TestCache_TTLExpiration and TestCache_CapacityLimit
}

func TestCache_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	cache := NewCache[int]("test", 1000, time.Hour)

	tmpDir := t.TempDir()

	// Create test files
	const numFiles = 100
	files := make([]string, numFiles)
	for i := 0; i < numFiles; i++ {
		files[i] = filepath.Join(tmpDir, "test"+string(rune('a'+i%26))+string(rune('0'+i/26))+".txt")
		require.NoError(t, os.WriteFile(files[i], []byte("content"), 0644))
	}

	// Concurrent writes
	done := make(chan bool)
	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			fi, _ := os.Stat(files[idx])
			cache.Store(files[idx], idx, fi)
			done <- true
		}(i)
	}

	for i := 0; i < numFiles; i++ {
		<-done
	}

	assert.Equal(t, numFiles, cache.Size())

	// Concurrent reads
	for i := 0; i < numFiles; i++ {
		go func(idx int) {
			_, _ = cache.Load(files[idx])
			done <- true
		}(i)
	}

	for i := 0; i < numFiles; i++ {
		<-done
	}
}

func TestCache_ZeroCapacityMeansUnlimited(t *testing.T) {
	t.Parallel()

	// Create cache with zero capacity (unlimited)
	cache := NewCache[string]("test", 0, time.Hour*24)

	tmpDir := t.TempDir()

	// Add many items
	for i := 0; i < 100; i++ {
		filePath := filepath.Join(tmpDir, "file"+string(rune('0'+i/10))+string(rune('0'+i%10))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

		fi, err := os.Stat(filePath)
		require.NoError(t, err)

		cache.Store(filePath, "data", fi)
	}

	assert.Equal(t, 100, cache.Size())

	// Run eviction - should not remove anything (no expiration, no capacity limit)
	cache.evict()

	assert.Equal(t, 100, cache.Size())
}

func TestCacheMetrics_Interface(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("metrics-test", 50, time.Hour)

	// Verify it implements CacheMetrics interface
	var metrics CacheMetrics = cache

	assert.Equal(t, "metrics-test", metrics.Name())
	assert.Equal(t, 0, metrics.Size())

	// Add an item
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
	fi, _ := os.Stat(filePath)
	cache.Store(filePath, "data", fi)

	assert.Equal(t, 1, metrics.Size())
}

func TestCache_StoreSameKeyTwice(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))

	fi, err := os.Stat(filePath)
	require.NoError(t, err)

	// Store same key twice
	cache.Store(filePath, "first", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Store(filePath, "second", fi)
	assert.Equal(t, 1, cache.Size(), "items counter should not increment for existing key")

	// Verify the value was updated
	data, ok := cache.Load(filePath)
	assert.True(t, ok)
	assert.Equal(t, "second", data)
}

func TestCache_InvalidateNonExistent(t *testing.T) {
	t.Parallel()

	cache := NewCache[string]("test", 100, time.Hour)

	// Invalidate a key that doesn't exist
	cache.Invalidate("non-existent")

	assert.Equal(t, 0, cache.Size(), "items counter should stay at 0")

	// Add one item, then invalidate twice
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
	fi, _ := os.Stat(filePath)

	cache.Store(filePath, "data", fi)
	assert.Equal(t, 1, cache.Size())

	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size())

	// Invalidate same key again
	cache.Invalidate(filePath)
	assert.Equal(t, 0, cache.Size(), "items counter should not go negative")
}

func TestCache_MixedExpirationAndCapacity(t *testing.T) {
	t.Parallel()

	// Cache with capacity of 3
	cache := NewCache[string]("test", 3, time.Hour)

	// Add 2 expired entries directly
	for i := 0; i < 2; i++ {
		expiredEntry := Entry[string]{
			Data:         "expired",
			Size:         10,
			LastModified: time.Now().Unix(),
			ExpiresAt:    time.Now().Add(-time.Hour),
		}
		cache.entries.Store("expired-"+string(rune('0'+i)), expiredEntry)
		cache.items.Add(1)
	}

	// Add 4 valid entries via Store (these will have future expiration)
	tmpDir := t.TempDir()
	for i := 0; i < 4; i++ {
		filePath := filepath.Join(tmpDir, "valid"+string(rune('0'+i))+".txt")
		require.NoError(t, os.WriteFile(filePath, []byte("content"), 0644))
		fi, _ := os.Stat(filePath)
		cache.Store(filePath, "valid", fi)
	}

	// Total: 2 expired + 4 valid = 6 items
	assert.Equal(t, 6, cache.Size())

	// Run eviction
	cache.evict()

	// Should remove 2 expired first, then evict excess to reach capacity of 3
	// Result: at most 3 items (could be exactly 3 if capacity eviction works)
	assert.LessOrEqual(t, cache.Size(), 3)

	// Verify no expired entries remain
	cache.entries.Range(func(key, value any) bool {
		entry := value.(Entry[string])
		assert.True(t, time.Now().Before(entry.ExpiresAt), "Found expired entry after eviction")
		return true
	})
}
